package chainutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-genesis-init", a.owner.GetName()),
			Namespace: a.owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "temp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "priv-key",
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
					Args: []string{"init", "test",
						"--chain-id", params.ChainID,
						"--home", "/home/app",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
					},
				},
				{
					Name:    "load-priv-key",
					Image:   "busybox",
					Command: []string{"/bin/sh"},
					Args: []string{
						"-c",
						"cp /secrets/priv_validator_key.json /home/app/config/priv_validator_key.json",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
						{
							Name:      "priv-key",
							MountPath: "/secrets",
						},
					},
				},
				{
					Name:            "load-account",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args: []string{"keys", "add", "account", "--recover",
						"--keyring-backend", "test",
						"--home", "/home/app",
					},
					Stdin:     true,
					StdinOnce: true,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
					},
				},
				{
					Name:            "add-validator-account",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args: []string{"add-genesis-account", account.Address, strings.Join(params.Assets, ","),
						"--home", "/home/app",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
					},
				},
				{
					Name:    "set-unbonding-time",
					Image:   "apteno/alpine-jq",
					Command: []string{"sh", "-c"},
					Args: []string{
						fmt.Sprintf("jq '.app_state.staking.params.unbonding_time = %q' /home/app/config/genesis.json > /tmp/genesis.tmp && mv /tmp/genesis.tmp /home/app/config/genesis.json", params.UnbondingTime),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
					},
				},
				{
					Name:    "set-voting-period",
					Image:   "apteno/alpine-jq",
					Command: []string{"sh", "-c"},
					Args: []string{
						fmt.Sprintf("jq '.app_state.gov.voting_params.voting_period = %q' /home/app/config/genesis.json > /tmp/genesis.tmp && mv /tmp/genesis.tmp /home/app/config/genesis.json", params.VotingPeriod),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"cat"},
					Stdin:   true,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/home/app",
						},
					},
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
			Args: []string{"add-genesis-account", acc.Address, strings.Join(acc.Assets, ","),
				"--home", "/home/app",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/home/app",
				},
			},
		})
	}

	// Add additional commands
	for i, cmd := range initCommands {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
			Name:    fmt.Sprintf("init-command-%d", i),
			Image:   cmd.Image,
			Command: cmd.Command,
			Args:    cmd.Args,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/home/app",
				},
				{
					Name:      "temp",
					MountPath: "/temp",
				},
			},
		})
	}

	// Add gentxs container
	infoArgs := []string{
		"--moniker", nodeInfo.Moniker,
	}
	if nodeInfo.Details != nil {
		infoArgs = append(infoArgs, "--details", *nodeInfo.Details)
	}
	if nodeInfo.Website != nil {
		infoArgs = append(infoArgs, "--website", *nodeInfo.Website)
	}
	if nodeInfo.Identity != nil {
		infoArgs = append(infoArgs, "--identity", *nodeInfo.Identity)
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            "gentx",
		Image:           a.image,
		ImagePullPolicy: a.pullPolicy,
		Command:         []string{a.binary},
		Args: append([]string{"gentx", "account", params.StakeAmount,
			"--chain-id", params.ChainID,
			"--home", "/home/app",
			"--keyring-backend", "test",
			"--yes",
		}, infoArgs...),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/home/app",
			},
		},
	})

	// Add collect-gentxs container
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            "collect-gentxs",
		Image:           a.image,
		ImagePullPolicy: a.pullPolicy,
		Command:         []string{a.binary},
		Args: []string{"collect-gentxs",
			"--home", "/home/app",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/home/app",
			},
		},
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

	genesis, _, err := ph.Exec(ctx, "busybox", []string{"cat", "/home/app/config/genesis.json"})
	return genesis, err
}
