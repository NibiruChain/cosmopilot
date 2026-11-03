package k8s

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func TestPodHelperDeleteUsesObservedUIDPrecondition(t *testing.T) {
	observedUID := types.UID("observed-pod")
	replacementPresent := true
	var requestedUID types.UID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodDelete, req.Method)
		require.Equal(t, "/api/v1/namespaces/default/pods/node", req.URL.Path)
		var options metav1.DeleteOptions
		require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
		if options.Preconditions != nil && options.Preconditions.UID != nil {
			requestedUID = *options.Preconditions.UID
		}
		if requestedUID != observedUID {
			replacementPresent = false
			writeJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
			return
		}
		writeJSON(t, w, http.StatusConflict, &metav1.Status{
			Status:  metav1.StatusFailure,
			Reason:  metav1.StatusReasonConflict,
			Code:    http.StatusConflict,
			Message: "pod UID precondition failed",
		})
	}))
	defer server.Close()
	helper := NewPodHelper(testClientSet(t, server.URL), nil, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: observedUID},
	})

	err := helper.Delete(t.Context())
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err))
	assert.Equal(t, observedUID, requestedUID)
	assert.True(t, replacementPresent, "replacement Pod must not be deleted")
}

func TestPodHelperDeleteWithGracePeriodUsesObservedUIDPrecondition(t *testing.T) {
	observedUID := types.UID("observed-pod")
	graceSeconds := int64(60)
	var options metav1.DeleteOptions
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, http.MethodDelete, req.Method)
		require.NoError(t, json.NewDecoder(req.Body).Decode(&options))
		writeJSON(t, w, http.StatusOK, &metav1.Status{Status: metav1.StatusSuccess})
	}))
	defer server.Close()
	helper := NewPodHelper(testClientSet(t, server.URL), nil, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: observedUID},
	})

	require.NoError(t, helper.DeleteWithGracePeriod(t.Context(), graceSeconds))
	require.NotNil(t, options.Preconditions)
	require.NotNil(t, options.Preconditions.UID)
	assert.Equal(t, observedUID, *options.Preconditions.UID)
	require.NotNil(t, options.GracePeriodSeconds)
	assert.Equal(t, graceSeconds, *options.GracePeriodSeconds)
}

func TestPodHelperWaitForPodDeletedReturnsWhenNameHasDifferentUID(t *testing.T) {
	observed := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "node", Namespace: "default", UID: "observed-pod"}}
	replacement := observed.DeepCopy()
	replacement.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"}
	replacement.UID = "replacement-pod"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests.Add(1)
		switch req.URL.Path {
		case "/api/v1/namespaces/default/pods/node":
			writeJSON(t, w, http.StatusOK, replacement)
		case "/api/v1/namespaces/default/pods":
			if req.URL.Query().Get("watch") == "true" {
				<-req.Context().Done()
				return
			}
			writeJSON(t, w, http.StatusOK, &corev1.PodList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "PodList"},
				Items:    []corev1.Pod{*replacement},
			})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	helper := NewPodHelper(testClientSet(t, server.URL), nil, observed)

	err := helper.WaitForPodDeleted(t.Context(), 100*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int32(1), requests.Load(), "UID change should finish after the initial live GET")
}

func testClientSet(t *testing.T, host string) *kubernetes.Clientset {
	t.Helper()
	clientSet, err := kubernetes.NewForConfig(&rest.Config{Host: host})
	require.NoError(t, err)
	return clientSet
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(value))
}
