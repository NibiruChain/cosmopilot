package chainutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/nibiru-operator/internal/chainutils/sdkargs"
	"github.com/NibiruChain/nibiru-operator/internal/k8s"
)

type GenesisParams struct {
	ChainID       string
	Assets        []string
	StakeAmount   string
	Accounts      []AccountAssets
	UnbondingTime string
	VotingPeriod  string
}

type NodeInfo struct {
	Moniker  string
	Details  *string
	Website  *string
	Identity *string
}

type AccountAssets struct {
	Address string
	Assets  []string
}

type InitCommand struct {
	Image   string
	Command []string
	Args    []string
}

func ExtractChainIdFromGenesis(genesis string) (string, error) {
	var genesisJson map[string]interface{}
	if err := json.Unmarshal([]byte(genesis), &genesisJson); err != nil {
		return "", err
	}
	if chainId, ok := genesisJson["chain_id"]; ok {
		return chainId.(string), nil
	}
	return "", fmt.Errorf("could not extract chain id from genesis")
}

func (a *App) NewGenesis(ctx context.Context,
	privkeySecret string,
	account *Account,
	nodeInfo *NodeInfo,
	params *GenesisParams,
	initCommands ...*InitCommand,
) (string, error) {

	var (
		dataVolumeMount = corev1.VolumeMount{
			Name:      "data",
			MountPath: defaultHome,
		}
		privKeyVolumeMount = corev1.VolumeMount{
			Name:      "priv-key",
			MountPath: "/secrets",
		}
		tempVolumeMount = corev1.VolumeMount{
			Name:      "temp",
			MountPath: "/temp",
		}
	)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-genesis-init", a.owner.GetName()),
			Namespace: a.owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: dataVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: tempVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: privKeyVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: privkeySecret,
						},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "init-chain",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args:            a.sdk.InitArgs(nodeInfo.Moniker, params.ChainID),
					VolumeMounts:    []corev1.VolumeMount{dataVolumeMount},
				},
				{
					Name:         "load-priv-key",
					Image:        "busybox",
					Command:      []string{"/bin/sh"},
					Args:         []string{"-c", "cp /secrets/priv_validator_key.json /home/app/config/priv_validator_key.json"},
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount, privKeyVolumeMount},
				},
				{
					Name:            "load-account",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args:            a.sdk.RecoverAccountArgs(defaultAccountName),
					Stdin:           true,
					StdinOnce:       true,
					VolumeMounts:    []corev1.VolumeMount{dataVolumeMount},
				},
				{
					Name:            "add-validator-account",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args:            a.sdk.AddGenesisAccountArgs(account.Address, params.Assets),
					VolumeMounts:    []corev1.VolumeMount{dataVolumeMount},
				},
				{
					Name:         "set-unbonding-time",
					Image:        "apteno/alpine-jq",
					Command:      []string{"sh", "-c"},
					Args:         []string{a.sdk.GenesisSetUnbondingTimeCmd(params.UnbondingTime, filepath.Join(defaultHome, defaultGenesisFile))},
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
				},
				{
					Name:         "set-voting-period",
					Image:        "apteno/alpine-jq",
					Command:      []string{"sh", "-c"},
					Args:         []string{a.sdk.GenesisSetVotingPeriodCmd(params.VotingPeriod, filepath.Join(defaultHome, defaultGenesisFile))},
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
				},
			},
			Containers: []corev1.Container{
				{
					Name:         "busybox",
					Image:        "busybox",
					Command:      []string{"cat"},
					Stdin:        true,
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
				},
			},
			TerminationGracePeriodSeconds: pointer.Int64(0),
		},
	}

	// Add additional accounts
	for i, acc := range params.Accounts {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:            fmt.Sprintf("add-account-%d", i),
			Image:           a.image,
			ImagePullPolicy: a.pullPolicy,
			Command:         []string{a.binary},
			Args:            a.sdk.AddGenesisAccountArgs(acc.Address, acc.Assets),
			VolumeMounts:    []corev1.VolumeMount{dataVolumeMount},
		})
	}

	// Add additional commands
	for i, cmd := range initCommands {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:         fmt.Sprintf("init-command-%d", i),
			Image:        cmd.Image,
			Command:      cmd.Command,
			Args:         cmd.Args,
			VolumeMounts: []corev1.VolumeMount{dataVolumeMount, tempVolumeMount},
		})
	}

	// Add gentx container
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            "gentx",
		Image:           a.image,
		ImagePullPolicy: a.pullPolicy,
		Command:         []string{a.binary},
		Args: a.sdk.GenTxArgs(defaultAccountName, nodeInfo.Moniker, params.StakeAmount, params.ChainID,
			sdkargs.WithOptionalArg(sdkargs.Details, nodeInfo.Details),
			sdkargs.WithOptionalArg(sdkargs.Website, nodeInfo.Website),
			sdkargs.WithOptionalArg(sdkargs.Identity, nodeInfo.Identity),
		),
		VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
	})

	// Add collect-gentxs container
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            "collect-gentxs",
		Image:           a.image,
		ImagePullPolicy: a.pullPolicy,
		Command:         []string{a.binary},
		Args:            a.sdk.CollectGenTxsArgs(),
		VolumeMounts:    []corev1.VolumeMount{dataVolumeMount},
	})

	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return "", err
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer ph.Delete(ctx)

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return "", err
	}

	// Wait for load-account container to be running
	if err := ph.WaitForInitContainerRunning(ctx, "load-account", time.Minute); err != nil {
		return "", err
	}

	// Attach to load-account container to insert mnemonic
	var input bytes.Buffer
	input.WriteString(fmt.Sprintf("%s\n", account.Mnemonic))
	if _, _, err := ph.Attach(ctx, "load-account", &input); err != nil {
		return "", err
	}

	// Wait for the pod to be running
	if err := ph.WaitForPodRunning(ctx, time.Minute); err != nil {
		return "", err
	}

	genesis, _, err := ph.Exec(ctx, "busybox", []string{"cat", filepath.Join(defaultHome, defaultGenesisFile)})
	return genesis, err
}
