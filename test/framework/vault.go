package framework

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// VaultNamespace is the namespace where Vault is deployed
	VaultNamespace = "vault-system"

	// VaultTokenSecretName is the name of the secret containing the Vault token for TMKMS
	VaultTokenSecretName = "vault-tmkms-token"

	// VaultCASecretName is the name of the secret containing the Vault CA certificate
	VaultCASecretName = "vault-tls"

	// VaultAddress returns the in-cluster Vault address
	VaultAddress = "https://vault.vault-system.svc.cluster.local:8200"
)

// vaultManifest is the Vault deployment manifest
const vaultManifest = `---
apiVersion: v1
kind: Namespace
metadata:
  name: vault-system
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vault-tls
  namespace: vault-system
spec:
  secretName: vault-tls
  duration: 8760h
  renewBefore: 720h
  issuerRef:
    name: cosmopilot-e2e
    kind: ClusterIssuer
  dnsNames:
    - vault
    - vault.vault-system
    - vault.vault-system.svc
    - vault.vault-system.svc.cluster.local
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: vault-config
  namespace: vault-system
data:
  vault.hcl: |
    listener "tcp" {
      address       = "0.0.0.0:8200"
      tls_cert_file = "/vault/tls/tls.crt"
      tls_key_file  = "/vault/tls/tls.key"
    }
    storage "file" {
      path = "/vault/data"
    }
    api_addr = "https://vault.vault-system.svc.cluster.local:8200"
    disable_mlock = true
---
apiVersion: v1
kind: Service
metadata:
  name: vault
  namespace: vault-system
spec:
  selector:
    app: vault
  ports:
    - port: 8200
      targetPort: 8200
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vault
  namespace: vault-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vault
  template:
    metadata:
      labels:
        app: vault
    spec:
      securityContext:
        fsGroup: 1000
      containers:
      - name: vault
        image: hashicorp/vault:1.15
        ports:
        - containerPort: 8200
        env:
        - name: VAULT_ADDR
          value: "https://127.0.0.1:8200"
        - name: VAULT_CACERT
          value: "/vault/tls/ca.crt"
        command:
        - vault
        - server
        - -config=/vault/config/vault.hcl
        volumeMounts:
        - name: vault-tls
          mountPath: /vault/tls
          readOnly: true
        - name: config
          mountPath: /vault/config
        - name: data
          mountPath: /vault/data
        securityContext:
          capabilities:
            add:
            - IPC_LOCK
        readinessProbe:
          exec:
            command:
            - sh
            - -c
            - "vault status -tls-skip-verify 2>&1 | grep -E '(Sealed\\s+false|Initialized\\s+false)'"
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: vault-tls
        secret:
          secretName: vault-tls
      - name: config
        configMap:
          name: vault-config
      - name: data
        emptyDir: {}
`

// tmkmsPolicy is the Vault policy for TMKMS operations
const tmkmsPolicy = `
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
path "auth/token/renew-self" {
  capabilities = ["update"]
}
path "transit/wrapping_key" {
  capabilities = ["read"]
}
path "transit/keys/*" {
  capabilities = ["create", "read", "update", "delete", "list"]
}
path "transit/sign/*" {
  capabilities = ["create", "update"]
}
path "transit/verify/*" {
  capabilities = ["create", "update"]
}
path "transit/export/signing-key/*" {
  capabilities = ["read"]
}
`

// vaultInitResponse represents the JSON response from vault operator init
type vaultInitResponse struct {
	UnsealKeysB64 []string `json:"unseal_keys_b64"`
	RootToken     string   `json:"root_token"`
}

// vaultTokenCreateResponse represents the JSON response from vault token create
type vaultTokenCreateResponse struct {
	Auth struct {
		ClientToken string `json:"client_token"`
	} `json:"auth"`
}

// installVault installs and configures HashiCorp Vault for TMKMS testing
func (f *KindFramework) installVault() error {
	log := logf.Log.WithName("kind-framework")

	// Ensure ClusterIssuer exists for certificate generation
	// This is normally created in suite_test.go after Setup(), but Vault needs it during Setup()
	log.Info("Ensuring ClusterIssuer exists for Vault TLS")
	if err := f.CreateClusterIssuer("cosmopilot-e2e"); err != nil {
		// Ignore error if already exists
		log.Info("ClusterIssuer may already exist", "error", err)
	}

	// Apply Vault manifests
	log.Info("Deploying Vault")
	if err := f.kubectlApplyServerSide(vaultManifest); err != nil {
		return fmt.Errorf("failed to apply vault manifests: %w", err)
	}

	// Wait for TLS certificate to be ready
	log.Info("Waiting for Vault TLS certificate")
	if err := f.kubectl("wait", "--for=condition=ready", "certificate",
		"vault-tls", "-n", VaultNamespace, "--timeout=120s"); err != nil {
		return fmt.Errorf("failed waiting for vault certificate: %w", err)
	}

	// Wait for Vault pod to be running (not ready yet - it's sealed)
	log.Info("Waiting for Vault pod to start")
	if err := f.waitForVaultPodRunning(); err != nil {
		return fmt.Errorf("failed waiting for vault pod: %w", err)
	}

	// Initialize and unseal Vault
	log.Info("Initializing Vault")
	rootToken, err := f.initializeVault()
	if err != nil {
		return fmt.Errorf("failed to initialize vault: %w", err)
	}

	// Wait for Vault to be ready after unsealing
	log.Info("Waiting for Vault to be ready")
	if err := f.kubectl("wait", "--for=condition=ready", "pod",
		"-l", "app=vault", "-n", VaultNamespace, "--timeout=120s"); err != nil {
		return fmt.Errorf("failed waiting for vault ready: %w", err)
	}

	// Configure Transit engine
	log.Info("Configuring Vault Transit engine")
	if err := f.configureVaultTransit(rootToken); err != nil {
		return fmt.Errorf("failed to configure vault transit: %w", err)
	}

	// Create TMKMS policy and token
	log.Info("Creating TMKMS policy and token")
	if err := f.createTmkmsToken(rootToken); err != nil {
		return fmt.Errorf("failed to create tmkms token: %w", err)
	}

	log.Info("Vault installation complete")
	return nil
}

// waitForVaultPodRunning waits for the Vault pod to be in Running phase
func (f *KindFramework) waitForVaultPodRunning() error {
	for i := 0; i < 60; i++ {
		pods, err := f.kubeClient.CoreV1().Pods(VaultNamespace).List(f.ctx, metav1.ListOptions{
			LabelSelector: "app=vault",
		})
		if err != nil {
			return err
		}
		if len(pods.Items) > 0 && pods.Items[0].Status.Phase == corev1.PodRunning {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for vault pod to be running")
}

// getVaultPodName returns the name of the Vault pod
func (f *KindFramework) getVaultPodName() (string, error) {
	pods, err := f.kubeClient.CoreV1().Pods(VaultNamespace).List(f.ctx, metav1.ListOptions{
		LabelSelector: "app=vault",
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no vault pods found")
	}
	return pods.Items[0].Name, nil
}

// VaultCredsSecretName is the name of the secret containing Vault root credentials
const VaultCredsSecretName = "vault-creds"

// initializeVault initializes and unseals Vault, returns the root token
func (f *KindFramework) initializeVault() (string, error) {
	podName, err := f.getVaultPodName()
	if err != nil {
		return "", err
	}

	// Check if already initialized
	statusOut, _ := f.PodExec(VaultNamespace, podName, "vault",
		"vault", "status", "-tls-skip-verify", "-format=json")
	if strings.Contains(statusOut, `"initialized": true`) {
		// Already initialized, try to get credentials from secret
		var rootToken, unsealKey string

		// Try the dedicated creds secret first
		secret, err := f.kubeClient.CoreV1().Secrets(VaultNamespace).Get(f.ctx, VaultCredsSecretName, metav1.GetOptions{})
		if err == nil {
			rootToken = string(secret.Data["root-token"])
			unsealKey = string(secret.Data["unseal-key"])
		}

		// Fallback to token secret (legacy location)
		if rootToken == "" {
			tokenSecret, err := f.kubeClient.CoreV1().Secrets(VaultNamespace).Get(f.ctx, VaultTokenSecretName, metav1.GetOptions{})
			if err == nil {
				if rt, ok := tokenSecret.Data["root-token"]; ok {
					rootToken = string(rt)
				}
				if uk, ok := tokenSecret.Data["unseal-key"]; ok {
					unsealKey = string(uk)
				}
			}
		}

		// Unseal if needed
		if strings.Contains(statusOut, `"sealed": true`) && unsealKey != "" {
			_, err = f.PodExec(VaultNamespace, podName, "vault",
				"vault", "operator", "unseal", "-tls-skip-verify", unsealKey)
			if err != nil {
				return "", fmt.Errorf("failed to unseal vault: %w", err)
			}
		}

		if rootToken != "" {
			return rootToken, nil
		}

		return "", fmt.Errorf("vault already initialized but no credentials available (delete vault-system namespace to reinitialize)")
	}

	// Initialize Vault with single key share
	initOut, err := f.PodExec(VaultNamespace, podName, "vault",
		"vault", "operator", "init", "-tls-skip-verify",
		"-key-shares=1", "-key-threshold=1", "-format=json")
	if err != nil {
		return "", fmt.Errorf("failed to initialize vault: %w", err)
	}

	var initResp vaultInitResponse
	if err := json.Unmarshal([]byte(initOut), &initResp); err != nil {
		return "", fmt.Errorf("failed to parse init response: %w", err)
	}

	if len(initResp.UnsealKeysB64) == 0 {
		return "", fmt.Errorf("no unseal keys returned")
	}

	unsealKey := initResp.UnsealKeysB64[0]
	rootToken := initResp.RootToken

	// Store credentials immediately so we can unseal on restart
	credsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VaultCredsSecretName,
			Namespace: VaultNamespace,
		},
		StringData: map[string]string{
			"unseal-key": unsealKey,
			"root-token": rootToken,
		},
	}
	_, err = f.kubeClient.CoreV1().Secrets(VaultNamespace).Create(f.ctx, credsSecret, metav1.CreateOptions{})
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return "", fmt.Errorf("failed to store vault credentials: %w", err)
	}

	// Unseal Vault
	_, err = f.PodExec(VaultNamespace, podName, "vault",
		"vault", "operator", "unseal", "-tls-skip-verify", unsealKey)
	if err != nil {
		return "", fmt.Errorf("failed to unseal vault: %w", err)
	}

	return rootToken, nil
}

// configureVaultTransit enables the Transit secrets engine
func (f *KindFramework) configureVaultTransit(rootToken string) error {
	podName, err := f.getVaultPodName()
	if err != nil {
		return err
	}

	// Enable Transit engine using VAULT_TOKEN env var (ignore error if already enabled)
	_, _ = f.PodExec(VaultNamespace, podName, "vault",
		"sh", "-c", fmt.Sprintf("VAULT_TOKEN=%s vault secrets enable -tls-skip-verify transit", rootToken))

	return nil
}

// createTmkmsToken creates the TMKMS policy and generates a token
func (f *KindFramework) createTmkmsToken(rootToken string) error {
	podName, err := f.getVaultPodName()
	if err != nil {
		return err
	}

	// Always update the policy (in case it changed)
	// Write policy to a temp file in the container, then apply it using VAULT_TOKEN env var
	policyCmd := fmt.Sprintf(`export VAULT_TOKEN=%s
cat > /tmp/tmkms-policy.hcl << 'EOF'
%s
EOF
vault policy write -tls-skip-verify tmkms /tmp/tmkms-policy.hcl`, rootToken, strings.TrimSpace(tmkmsPolicy))

	_, err = f.PodExec(VaultNamespace, podName, "vault",
		"sh", "-c", policyCmd)
	if err != nil {
		return fmt.Errorf("failed to create tmkms policy: %w", err)
	}

	// Check if token secret already exists with valid data
	existingSecret, getErr := f.kubeClient.CoreV1().Secrets(VaultNamespace).Get(f.ctx, VaultTokenSecretName, metav1.GetOptions{})
	if getErr == nil && len(existingSecret.Data["token"]) > 0 {
		// Token secret already exists and has a token, skip token creation
		// (policy was already updated above)
		return nil
	}

	// Create token with policy using VAULT_TOKEN env var
	tokenOut, err := f.PodExec(VaultNamespace, podName, "vault",
		"sh", "-c", fmt.Sprintf("VAULT_TOKEN=%s vault token create -tls-skip-verify -policy=tmkms -format=json", rootToken))
	if err != nil {
		return fmt.Errorf("failed to create tmkms token: %w", err)
	}

	var tokenResp vaultTokenCreateResponse
	if err := json.Unmarshal([]byte(tokenOut), &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	tmkmsToken := tokenResp.Auth.ClientToken

	// Store token in a secret (create or update)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VaultTokenSecretName,
			Namespace: VaultNamespace,
		},
		StringData: map[string]string{
			"token":      tmkmsToken,
			"root-token": rootToken, // Store root token for debugging/reuse
		},
	}

	// If secret exists (even without valid token data), update it; otherwise create
	if getErr == nil {
		secret.ResourceVersion = existingSecret.ResourceVersion
		_, err = f.kubeClient.CoreV1().Secrets(VaultNamespace).Update(f.ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update token secret: %w", err)
		}
	} else {
		_, err = f.kubeClient.CoreV1().Secrets(VaultNamespace).Create(f.ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create token secret: %w", err)
		}
	}

	return nil
}
