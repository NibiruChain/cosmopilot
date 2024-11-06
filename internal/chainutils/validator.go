package chainutils

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/NibiruChain/cosmopilot/internal/chainutils/sdkcmd"
	"github.com/NibiruChain/cosmopilot/internal/k8s"
)

func (a *App) CreateValidator(
	ctx context.Context,
	pubKey string,
	account *Account,
	nodeInfo *NodeInfo,
	params *Params,
	node string,
) error {

	var (
		dataVolumeMount = corev1.VolumeMount{
			Name:      "data",
			MountPath: defaultHome,
		}
	)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-create-validator", a.owner.GetName()),
			Namespace: a.owner.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:     corev1.RestartPolicyNever,
			PriorityClassName: a.priorityClassName,
			Affinity:          a.Affinity,
			NodeSelector:      a.NodeSelector,
			Volumes: []corev1.Volume{
				{
					Name: dataVolumeMount.Name,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "load-account",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args:            a.cmd.RecoverAccountArgs(defaultAccountName),
					Stdin:           true,
					StdinOnce:       true,
					VolumeMounts:    []corev1.VolumeMount{dataVolumeMount},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "create-validator",
					Image:           a.image,
					ImagePullPolicy: a.pullPolicy,
					Command:         []string{a.binary},
					Args: a.cmd.CreateValidatorArgs(
						defaultAccountName,
						pubKey,
						nodeInfo.Moniker,
						params.StakeAmount,
						params.ChainID,
						params.GasPrices,
						sdkcmd.WithArg(sdkcmd.CommissionMaxChangeRate, params.CommissionMaxChangeRate),
						sdkcmd.WithArg(sdkcmd.CommissionMaxRate, params.CommissionMaxRate),
						sdkcmd.WithArg(sdkcmd.CommissionRate, params.CommissionRate),
						sdkcmd.WithOptionalArg(sdkcmd.MinSelfDelegation, params.MinSelfDelegation),
						sdkcmd.WithOptionalArg(sdkcmd.Details, nodeInfo.Details),
						sdkcmd.WithOptionalArg(sdkcmd.Website, nodeInfo.Website),
						sdkcmd.WithOptionalArg(sdkcmd.Identity, nodeInfo.Identity),
						sdkcmd.WithArg(sdkcmd.Node, node),
					),
					VolumeMounts: []corev1.VolumeMount{dataVolumeMount},
				},
			},
			TerminationGracePeriodSeconds: pointer.Int64(0),
		},
	}

	if err := controllerutil.SetControllerReference(a.owner, pod, a.scheme); err != nil {
		return err
	}

	ph := k8s.NewPodHelper(a.client, a.restConfig, pod)

	// Delete the pod if it already exists
	_ = ph.Delete(ctx)

	// Delete the pod independently of the result
	defer ph.Delete(ctx)

	// Create the pod
	if err := ph.Create(ctx); err != nil {
		return err
	}

	// Wait for load-account container to be running
	if err := ph.WaitForInitContainerRunning(ctx, "load-account", time.Minute); err != nil {
		return err
	}

	// Attach to load-account container to insert mnemonic
	var input bytes.Buffer
	input.WriteString(fmt.Sprintf("%s\n", account.Mnemonic))
	if _, _, err := ph.Attach(ctx, "load-account", &input); err != nil {
		return err
	}

	// Wait for the pod to be completed
	return ph.WaitForPodSucceeded(ctx, time.Minute)
}
