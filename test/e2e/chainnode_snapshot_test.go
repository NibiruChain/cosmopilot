package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
)

var _ = Describe("Snapshot E2E", func() {
	Context("Volume Snapshots", func() {
		apps.ForEachApp("should create snapshots via persistence.snapshots config",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNode := app.BuildChainNode(ns.Name)
				configureSnapshotPersistence(chainNode)

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for node to be running and generate some blocks
				WaitForChainNodeHeight(chainNode, 5)
				RefreshChainNode(chainNode)

				// Wait for cosmopilot to create at least one snapshot
				By("Waiting for cosmopilot to create a snapshot")
				Eventually(func() int {
					return countSnapshotsForPVC(ns.Name, chainNode.Name)
				}, 2*time.Minute, 10*time.Second).Should(BeNumerically(">=", 1))
			}),
		)

		apps.ForEachApp("should restore a ChainNode from a snapshot",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNode := app.BuildChainNode(ns.Name)
				configureSnapshotPersistence(chainNode)

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for node to generate some blocks
				WaitForChainNodeHeight(chainNode, 5)

				// Wait for cosmopilot to create at least one ready snapshot
				By("Waiting for cosmopilot to create a snapshot")
				var snapshotName string
				Eventually(func() bool {
					snapshotName = findReadySnapshotForPVC(ns.Name, chainNode.Name)
					return snapshotName != ""
				}, 2*time.Minute, 10*time.Second).Should(BeTrue())

				// Delete the original ChainNode
				err = Framework().Client().Delete(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for ChainNode to be deleted
				Eventually(func() bool {
					err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNode), chainNode)
					return err != nil
				}).Should(BeTrue())

				// Create a new ChainNode that restores from the snapshot
				restoredChainNode := app.BuildChainNode(ns.Name)
				if restoredChainNode.Spec.Persistence == nil {
					restoredChainNode.Spec.Persistence = &appsv1.Persistence{}
				}
				restoredChainNode.Spec.Persistence.StorageClassName = ptr.To("csi-hostpath-sc")
				restoredChainNode.Spec.Persistence.RestoreFromSnapshot = &appsv1.PvcSnapshot{
					Name: snapshotName,
				}
				// Remove validator init since we're restoring from snapshot
				restoredChainNode.Spec.Validator = nil
				// Set genesis to use data volume since genesis is already in the snapshot
				restoredChainNode.Spec.Genesis = &appsv1.GenesisConfig{
					UseDataVolume: ptr.To(true),
					ChainID:       ptr.To(app.ValidatorConfig.ChainID),
				}

				err = Framework().Client().Create(Framework().Context(), restoredChainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for restored node to start generating blocks
				WaitForChainNodeHeight(restoredChainNode, 1)
			}),
		)
	})

	Context("Snapshot Retention", func() {
		apps.ForEachApp("should enforce retain count and delete old snapshots",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				chainNode := app.BuildChainNode(ns.Name)
				if chainNode.Spec.Persistence == nil {
					chainNode.Spec.Persistence = &appsv1.Persistence{}
				}
				chainNode.Spec.Persistence.StorageClassName = ptr.To("csi-hostpath-sc")
				chainNode.Spec.Persistence.Snapshots = &appsv1.VolumeSnapshotsConfig{
					Frequency:            "1m",             // Create snapshot every minute
					Retain:               ptr.To[int32](2), // Keep only 2 snapshots
					PreserveLastSnapshot: ptr.To(true),
					SnapshotClassName:    ptr.To("csi-hostpath-snapclass"),
				}

				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for node to start generating blocks
				WaitForChainNodeHeight(chainNode, 5)
				RefreshChainNode(chainNode)

				// Wait for at least 3 snapshot cycles to pass (3+ minutes)
				// This ensures retention has a chance to kick in
				By("Waiting for snapshots to be created and retention to be enforced")
				time.Sleep(4 * time.Minute)

				// Verify retention policy is enforced (should have exactly 2 snapshots)
				Eventually(func() int {
					return countSnapshotsForPVC(ns.Name, chainNode.Name)
				}, 2*time.Minute, 10*time.Second).Should(Equal(2), "Expected exactly 2 snapshots due to retain policy")

				// Verify the snapshots are ready
				snapshotList := &snapshotv1.VolumeSnapshotList{}
				err = Framework().Client().List(Framework().Context(), snapshotList, &client.ListOptions{
					Namespace: ns.Name,
				})
				Expect(err).NotTo(HaveOccurred())

				for _, snap := range snapshotList.Items {
					if snap.Spec.Source.PersistentVolumeClaimName != nil &&
						*snap.Spec.Source.PersistentVolumeClaimName == chainNode.Name {
						Expect(snap.Status).NotTo(BeNil())
						Expect(snap.Status.ReadyToUse).NotTo(BeNil())
						Expect(*snap.Status.ReadyToUse).To(BeTrue(), fmt.Sprintf("Snapshot %s should be ready", snap.Name))
					}
				}
			}),
		)
	})
})

// configureSnapshotPersistence sets up a ChainNode for snapshot testing
func configureSnapshotPersistence(chainNode *appsv1.ChainNode) {
	if chainNode.Spec.Persistence == nil {
		chainNode.Spec.Persistence = &appsv1.Persistence{}
	}
	chainNode.Spec.Persistence.StorageClassName = ptr.To("csi-hostpath-sc")
	chainNode.Spec.Persistence.Snapshots = &appsv1.VolumeSnapshotsConfig{
		Frequency:         "1m", // Create snapshot every minute
		SnapshotClassName: ptr.To("csi-hostpath-snapclass"),
	}
}

// countSnapshotsForPVC counts the number of snapshots for a given PVC
func countSnapshotsForPVC(namespace, pvcName string) int {
	snapshotList := &snapshotv1.VolumeSnapshotList{}
	if err := Framework().Client().List(Framework().Context(), snapshotList, &client.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return 0
	}

	count := 0
	for _, snap := range snapshotList.Items {
		if snap.Spec.Source.PersistentVolumeClaimName != nil &&
			*snap.Spec.Source.PersistentVolumeClaimName == pvcName {
			count++
		}
	}
	return count
}

// findReadySnapshotForPVC finds a ready snapshot for a given PVC and returns its name
func findReadySnapshotForPVC(namespace, pvcName string) string {
	snapshotList := &snapshotv1.VolumeSnapshotList{}
	if err := Framework().Client().List(Framework().Context(), snapshotList, &client.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return ""
	}

	for _, snap := range snapshotList.Items {
		if snap.Spec.Source.PersistentVolumeClaimName != nil &&
			*snap.Spec.Source.PersistentVolumeClaimName == pvcName &&
			snap.Status != nil &&
			snap.Status.ReadyToUse != nil &&
			*snap.Status.ReadyToUse {
			return snap.Name
		}
	}
	return ""
}
