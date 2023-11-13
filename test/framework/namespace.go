package framework

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	randomNamespacePrefix = "no-e2e-"
)

func (tf *TestFramework) CreateRandomNamespace() (*corev1.Namespace, error) {
	return tf.KubeClient.CoreV1().Namespaces().Create(tf.Context(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: randomNamespacePrefix,
		},
	}, metav1.CreateOptions{})
}

func (tf *TestFramework) DeleteNamespace(ns *corev1.Namespace) error {
	return tf.KubeClient.CoreV1().Namespaces().Delete(tf.Context(), ns.Name, metav1.DeleteOptions{
		GracePeriodSeconds: pointer.Int64(0),
	})
}
