package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"github.com/Dasmat13/kubecorrelate/pkg/watcher"
)

func TestManagerStartAndEventFlow(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	// Pre-create a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}
	_, err := clientset.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to pre-create pod: %v", err)
	}

	mgr := NewManager(clientset, nil, Config{
		Namespace:    "default",
		Since:        10 * time.Minute,
		BufferDelay:  50 * time.Millisecond,
		RelativeTime: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Run manager in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- mgr.Start(ctx)
	}()

	// Wait for start
	time.Sleep(50 * time.Millisecond)

	// Send an event
	mgr.eventChan <- watcher.TelemetryEvent{
		Timestamp: time.Now(),
		Type:      watcher.TypeLog,
		Namespace: "default",
		PodName:   "test-pod",
		Message:   "hello log message",
		Source:    "container",
	}

	err = <-errChan
	if err != nil && err != context.DeadlineExceeded && err.Error() != "context canceled" {
		t.Errorf("expected clean stop or context canceled, got: %v", err)
	}
}
