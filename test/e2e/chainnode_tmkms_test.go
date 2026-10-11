package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/voluzi/cosmopilot/v2/test/e2e/apps"
)

var _ = Describe("ChainNode TMKMS", func() {
	Context("Validator with TMKMS Vault Provider", func() {
		apps.ForEachApp("should start validator using TMKMS with Vault and produce blocks",
			WithNamespace(func(app apps.TestApp, ns *corev1.Namespace) {
				// Skip if Vault is not installed
				if !Framework().Config().InstallVault {
					Skip("Vault is not installed, skipping TMKMS test")
				}

				// Copy Vault secrets to the test namespace
				tokenSecretName, caSecretName := CopyVaultSecretsToNamespace(ns.Name)

				// Build ChainNode with TMKMS configuration
				vaultAddr := GetVaultAddress()
				keyName := fmt.Sprintf("%s-validator", app.ValidatorConfig.ChainID)

				tmkmsConfig := apps.TmKMSConfig{
					VaultAddress:    vaultAddr,
					KeyName:         keyName,
					TokenSecretName: tokenSecretName,
					CASecretName:    caSecretName,
				}

				chainNode := app.BuildChainNodeWithTmKMS(ns.Name, tmkmsConfig)

				// Create ChainNode
				err := Framework().Client().Create(Framework().Context(), chainNode)
				Expect(err).NotTo(HaveOccurred())

				// Wait for ChainNode to reach running phase
				WaitForChainNodeRunning(chainNode)

				// Verify blocks are being produced (validator is signing via TMKMS)
				WaitForChainNodeHeight(chainNode, 3)

				// Verify TMKMS container is healthy
				WaitForTmkmsContainerRunning(chainNode)

				// Verify key was uploaded to Vault
				RefreshChainNode(chainNode)
				Expect(chainNode.Annotations).To(HaveKey("cosmopilot.voluzi.com/vault-key-uploaded"))
			}),
		)
	})
})
