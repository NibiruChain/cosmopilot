package k8s

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	// NonRootUID is the standard non-root user ID used across all pods
	NonRootUID = 1000
)

// RestrictedSecurityContext returns a container security context that complies
// with the Kubernetes PodSecurity "restricted" profile.
func RestrictedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		RunAsNonRoot:             ptr.To(true),
		RunAsUser:                ptr.To[int64](NonRootUID),
		RunAsGroup:               ptr.To[int64](NonRootUID),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

// RestrictedPodSecurityContext returns a pod security context that complies
// with the Kubernetes PodSecurity "restricted" profile.
func RestrictedPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		RunAsUser:    ptr.To[int64](NonRootUID),
		RunAsGroup:   ptr.To[int64](NonRootUID),
		FSGroup:      ptr.To[int64](NonRootUID),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}
