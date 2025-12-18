package framework

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
)

// TestHelpers provides helper methods for testing
type TestHelpers struct {
	client client.Client
	ctx    context.Context
}

// NewTestHelpers creates a new TestHelpers instance
func NewTestHelpers(c client.Client, ctx context.Context) *TestHelpers {
	return &TestHelpers{
		client: c,
		ctx:    ctx,
	}
}

// SimulateCertificateReady simulates cert-manager creating a TLS secret for a Certificate
// This is useful for integration tests where cert-manager is not running
func (h *TestHelpers) SimulateCertificateReady(namespace, secretName string, dnsNames []string) error {
	// Generate a self-signed certificate
	certPEM, keyPEM, err := h.generateSelfSignedCert(dnsNames)
	if err != nil {
		return fmt.Errorf("failed to generate certificate: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       certPEM,
			corev1.TLSPrivateKeyKey: keyPEM,
			"ca.crt":                certPEM, // Self-signed, so CA is same as cert
		},
	}

	return h.client.Create(h.ctx, secret)
}

// SimulateSnapshotReady simulates a VolumeSnapshot becoming ready
// This is useful for integration tests where the CSI driver is not running
func (h *TestHelpers) SimulateSnapshotReady(namespace, snapshotName string, restoreSize string) error {
	snapshot := &snapshotv1.VolumeSnapshot{}
	if err := h.client.Get(h.ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      snapshotName,
	}, snapshot); err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Create VolumeSnapshotContent
	contentName := fmt.Sprintf("snapcontent-%s", snapshot.UID)
	content := &snapshotv1.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: contentName,
		},
		Spec: snapshotv1.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1.VolumeSnapshotContentDelete,
			Driver:         "hostpath.csi.k8s.io",
			Source: snapshotv1.VolumeSnapshotContentSource{
				SnapshotHandle: ptr.To(fmt.Sprintf("snapshot-%s", snapshot.UID)),
			},
			VolumeSnapshotRef: corev1.ObjectReference{
				APIVersion: "snapshot.storage.k8s.io/v1",
				Kind:       "VolumeSnapshot",
				Name:       snapshot.Name,
				Namespace:  snapshot.Namespace,
				UID:        snapshot.UID,
			},
		},
	}

	if err := h.client.Create(h.ctx, content); err != nil {
		return fmt.Errorf("failed to create snapshot content: %w", err)
	}

	// Update snapshot status
	snapshot.Status = &snapshotv1.VolumeSnapshotStatus{
		BoundVolumeSnapshotContentName: ptr.To(contentName),
		ReadyToUse:                     ptr.To(true),
		CreationTime:                   ptr.To(metav1.Now()),
	}

	if err := h.client.Status().Update(h.ctx, snapshot); err != nil {
		return fmt.Errorf("failed to update snapshot status: %w", err)
	}

	// Update content status
	content.Status = &snapshotv1.VolumeSnapshotContentStatus{
		ReadyToUse:     ptr.To(true),
		SnapshotHandle: ptr.To(fmt.Sprintf("snapshot-%s", snapshot.UID)),
		CreationTime:   ptr.To(time.Now().UnixNano()),
	}

	if err := h.client.Status().Update(h.ctx, content); err != nil {
		return fmt.Errorf("failed to update snapshot content status: %w", err)
	}

	return nil
}

// CreateVolumeSnapshotClass creates a VolumeSnapshotClass for testing
func (h *TestHelpers) CreateVolumeSnapshotClass(name, driver string) error {
	class := &snapshotv1.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Driver:         driver,
		DeletionPolicy: snapshotv1.VolumeSnapshotContentDelete,
	}

	return h.client.Create(h.ctx, class)
}

// CreateSecret creates a generic secret
func (h *TestHelpers) CreateSecret(namespace, name string, data map[string][]byte) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	return h.client.Create(h.ctx, secret)
}

// CreateConfigMap creates a ConfigMap
func (h *TestHelpers) CreateConfigMap(namespace, name string, data map[string]string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	return h.client.Create(h.ctx, cm)
}

// WaitForPodReady waits for a pod to be ready
func (h *TestHelpers) WaitForPodReady(namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pod := &corev1.Pod{}
		if err := h.client.Get(h.ctx, types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		}, pod); err != nil {
			time.Sleep(time.Second)
			continue
		}

		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return nil
			}
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("pod %s/%s did not become ready within %v", namespace, name, timeout)
}

// generateSelfSignedCert generates a self-signed certificate
func (h *TestHelpers) generateSelfSignedCert(dnsNames []string) (certPEM, keyPEM []byte, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		DNSNames:              dnsNames,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return certPEM, keyPEM, nil
}

// RandString generates a random string of the specified length
func RandString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		randByte := make([]byte, 1)
		rand.Read(randByte)
		b[i] = letterBytes[int(randByte[0])%len(letterBytes)]
	}
	return string(b)
}
