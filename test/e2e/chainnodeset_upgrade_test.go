package e2e

import (
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/voluzi/cosmopilot/v2/api/v1"
	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
)

var _ = Describe("ChainNodeSet Governance Upgrade", func() {
	Context("Software Upgrade via Governance", func() {
		for _, app := range apps.All() {
			app := app // capture range variable

			// Skip apps without upgrade test configuration
			if len(app.UpgradeTests) == 0 {
				continue
			}

			for _, upgradeTest := range app.UpgradeTests {
				upgradeTest := upgradeTest // capture range variable
				testName := fmt.Sprintf("should perform gov upgrade from %s to %s", upgradeTest.FromVersion, upgradeTest.ToVersion)

				It(testName+" ["+app.Name+"]", WithNs(func(ns *corev1.Namespace) {
					// Determine the target image
					targetImage := upgradeTest.ToImage
					if targetImage == "" {
						targetVersion := upgradeTest.ToVersion
						if targetVersion == "" {
							targetVersion = *app.AppSpec.Version
						}
						targetImage = fmt.Sprintf("%s:%s", app.AppSpec.Image, targetVersion)
					}

					// Create a ChainNodeSet with the older version: 1 validator + 1 fullnode
					chainNodeSet := app.BuildChainNodeSet(ns.Name, 1)
					chainNodeSet.Spec.App.Version = ptr.To(upgradeTest.FromVersion)

					err := Framework().Client().Create(Framework().Context(), chainNodeSet)
					Expect(err).NotTo(HaveOccurred())

					// Wait for validator pod to be ready so we can submit transactions
					By("Waiting for validator pod to be ready")
					WaitForPodReady(chainNodeSet.Namespace, chainNodeSet.Name+"-validator")

					// Refresh to get current height
					RefreshChainNodeSet(chainNodeSet)
					currentHeight := chainNodeSet.Status.LatestHeight
					upgradeHeight := currentHeight + 50

					// Build upgrade info JSON with docker image
					upgradeInfo := map[string]interface{}{
						"binaries": map[string]string{
							"docker": targetImage,
						},
					}
					upgradeInfoJSON, err := json.Marshal(upgradeInfo)
					Expect(err).NotTo(HaveOccurred())

					upgradeName := upgradeTest.UpgradeName
					if upgradeName == "" {
						upgradeName = "test-upgrade"
					}

					// Build image, account secret, and node endpoint
					appImage := fmt.Sprintf("%s:%s", app.AppSpec.Image, upgradeTest.FromVersion)
					accountSecretName := chainNodeSet.Name + "-validator-account"
					nodeEndpoint := fmt.Sprintf("tcp://%s-validator:26657", chainNodeSet.Name)

					// Submit upgrade proposal
					By(fmt.Sprintf("Submitting upgrade proposal '%s' at height %d with image %s", upgradeName, upgradeHeight, targetImage))
					sdkVersion := appsv1.V0_47
					if app.AppSpec.SdkVersion != nil {
						sdkVersion = *app.AppSpec.SdkVersion
					}
					submitArgs := buildUpgradeProposalArgs(
						upgradeName,
						upgradeHeight,
						string(upgradeInfoJSON),
						app.ValidatorConfig.Denom,
						app.ValidatorConfig.ChainID,
						nodeEndpoint,
						sdkVersion,
					)
					_, err = Framework().RunAppCommand(ns.Name, appImage, app.AppSpec.App, accountSecretName, submitArgs)
					Expect(err).NotTo(HaveOccurred())

					// Wait for proposal to be in voting period
					By("Waiting for proposal to be in voting period")
					waitForProposalVotingPeriod(ns.Name, appImage, app.AppSpec.App, accountSecretName, "1", app.ValidatorConfig.ChainID, nodeEndpoint, app.ValidatorConfig.SkipChainIDForQueries)

					// Vote yes on proposal (proposal ID is 1 for the first proposal)
					By("Voting yes on the upgrade proposal")
					voteArgs := buildVoteArgs("1", app.ValidatorConfig.Denom, app.ValidatorConfig.ChainID, nodeEndpoint)
					_, err = Framework().RunAppCommand(ns.Name, appImage, app.AppSpec.App, accountSecretName, voteArgs)
					Expect(err).NotTo(HaveOccurred())

					// Wait for cosmopilot to detect the upgrade from governance
					By("Waiting for cosmopilot to detect the on-chain upgrade")
					WaitForUpgradeScheduled(chainNodeSet, upgradeHeight)

					// Wait for upgrade to complete
					By("Waiting for upgrade to complete")
					WaitForUpgradeCompleted(chainNodeSet, upgradeHeight)

					// Verify nodes are running with new version and generating blocks
					By("Verifying nodes continue to produce blocks after upgrade")
					WaitForChainNodesHeight(chainNodeSet, upgradeHeight+5)

					// Verify all pods are running with the new image
					By("Verifying all pods have the new image")
					VerifyPodsHaveImage(ns.Name, chainNodeSet.Name, app.AppSpec.App, targetImage)
				}))
			}
		}
	})
})

// WaitForUpgradeScheduled waits for an upgrade to be scheduled at the given height
func WaitForUpgradeScheduled(chainNodeSet *appsv1.ChainNodeSet, upgradeHeight int64) {
	Eventually(func() bool {
		current := appsv1.ChainNodeSet{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), &current); err != nil {
			return false
		}
		for _, upgrade := range current.Status.Upgrades {
			if upgrade.Source == appsv1.OnChainUpgrade && upgrade.Height == upgradeHeight {
				return upgrade.Status == appsv1.UpgradeScheduled
			}
		}
		return false
	}).Should(BeTrue())
}

// WaitForUpgradeCompleted waits for an upgrade to be completed at the given height
func WaitForUpgradeCompleted(chainNodeSet *appsv1.ChainNodeSet, upgradeHeight int64) {
	Eventually(func() bool {
		current := appsv1.ChainNodeSet{}
		if err := Framework().Client().Get(Framework().Context(), client.ObjectKeyFromObject(chainNodeSet), &current); err != nil {
			return false
		}
		for _, upgrade := range current.Status.Upgrades {
			if upgrade.Height == upgradeHeight {
				return upgrade.Status == appsv1.UpgradeCompleted
			}
		}
		return false
	}).Should(BeTrue())
}

// VerifyPodsHaveImage checks that all pods with the given prefix have the expected image
func VerifyPodsHaveImage(namespace, podPrefix, containerName, expectedImage string) {
	podList := &corev1.PodList{}
	err := Framework().Client().List(Framework().Context(), podList, &client.ListOptions{
		Namespace: namespace,
	})
	Expect(err).NotTo(HaveOccurred())

	for _, pod := range podList.Items {
		if !strings.HasPrefix(pod.Name, podPrefix) {
			continue
		}
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				Expect(container.Image).To(Equal(expectedImage),
					fmt.Sprintf("Pod %s should have image %s", pod.Name, expectedImage))
			}
		}
	}
}

// buildUpgradeProposalArgs builds the args to submit an upgrade proposal.
// For SDK < v0.50, uses tx gov submit-legacy-proposal software-upgrade.
// For SDK >= v0.50, uses tx upgrade software-upgrade.
func buildUpgradeProposalArgs(upgradeName string, upgradeHeight int64, upgradeInfo, denom, chainID, nodeEndpoint string, sdkVersion appsv1.SdkVersion) []string {
	if sdkVersion == appsv1.V0_50 || sdkVersion == appsv1.V0_53 {
		// SDK v0.50+ uses tx upgrade software-upgrade
		return []string{
			"tx", "upgrade", "software-upgrade", upgradeName,
			"--upgrade-height", fmt.Sprintf("%d", upgradeHeight),
			"--upgrade-info", upgradeInfo,
			"--title", upgradeName,
			"--summary", "Test upgrade",
			"--deposit", fmt.Sprintf("10000000%s", denom),
			"--from", "account",
			"--keyring-backend", "test",
			"--home", "/home/app",
			"--chain-id", chainID,
			"--node", nodeEndpoint,
			"--yes",
			"--no-validate",
			"--gas", "500000",
			"--fees", fmt.Sprintf("500000%s", denom),
		}
	}

	// SDK < v0.50 uses tx gov submit-legacy-proposal software-upgrade
	return []string{
		"tx", "gov", "submit-legacy-proposal", "software-upgrade", upgradeName,
		"--upgrade-height", fmt.Sprintf("%d", upgradeHeight),
		"--upgrade-info", upgradeInfo,
		"--title", upgradeName,
		"--description", "Test upgrade",
		"--deposit", fmt.Sprintf("10000000%s", denom),
		"--from", "account",
		"--keyring-backend", "test",
		"--home", "/home/app",
		"--chain-id", chainID,
		"--node", nodeEndpoint,
		"--yes",
		"--no-validate",
		"--gas", "500000",
		"--fees", fmt.Sprintf("50000%s", denom),
	}
}

// buildVoteArgs builds the args to vote yes on a proposal
func buildVoteArgs(proposalID, denom, chainID, nodeEndpoint string) []string {
	return []string{
		"tx", "gov", "vote", proposalID, "yes",
		"--from", "account",
		"--keyring-backend", "test",
		"--home", "/home/app",
		"--chain-id", chainID,
		"--node", nodeEndpoint,
		"--yes",
		"--fees", fmt.Sprintf("1000000%s", denom),
	}
}

// waitForProposalVotingPeriod polls the chain until the proposal is in voting period
func waitForProposalVotingPeriod(namespace, appImage, appName, accountSecretName, proposalID, chainID, nodeEndpoint string, skipChainID bool) {
	queryArgs := []string{
		"query", "gov", "proposal", proposalID,
		"--node", nodeEndpoint,
		"--output", "json",
	}
	if !skipChainID {
		queryArgs = append(queryArgs, "--chain-id", chainID)
	}
	Eventually(func() bool {
		output, err := Framework().RunAppCommand(namespace, appImage, appName, accountSecretName, queryArgs)
		if err != nil {
			return false
		}
		// Check if proposal is in voting period by looking for status field
		// PROPOSAL_STATUS_VOTING_PERIOD is status 2 in cosmos-sdk
		return strings.Contains(output, "PROPOSAL_STATUS_VOTING_PERIOD") || strings.Contains(output, `"status":"2"`) || strings.Contains(output, `"status": "2"`)
	}).Should(BeTrue())
}
