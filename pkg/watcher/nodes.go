package watcher

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

type NodeWatcher struct {
	client   kubernetes.Interface
	nodeName string
}

func NewNodeWatcher(client kubernetes.Interface, nodeName string) *NodeWatcher {
	return &NodeWatcher{
		client:   client,
		nodeName: nodeName,
	}
}

func (w *NodeWatcher) Watch(ctx context.Context, eventChan chan<- TelemetryEvent) error {
	// Only fetch events for this specific node
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Node", w.nodeName)
	opts := metav1.ListOptions{
		FieldSelector: fieldSelector,
		Watch:         true,
	}

	watcher, err := w.client.CoreV1().Events("").Watch(ctx, opts)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case eventObj, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("node event watcher closed unexpectedly")
			}

			if eventObj.Type == watch.Error {
				continue
			}

			k8sEvent, ok := eventObj.Object.(*corev1.Event)
			if !ok {
				continue
			}

			eventTime := k8sEvent.LastTimestamp.Time
			if eventTime.IsZero() {
				eventTime = k8sEvent.FirstTimestamp.Time
			}
			if eventTime.IsZero() {
				eventTime = time.Now()
			}

			// Skip older events (we are interested in current active events)
			if time.Since(eventTime) > 1*time.Minute {
				continue
			}

			severity := "INFO"
			if k8sEvent.Type == "Warning" {
				severity = "WARN"
			}

			msg := fmt.Sprintf("[%s] %s: %s", severity, k8sEvent.Reason, k8sEvent.Message)

			eventChan <- TelemetryEvent{
				Timestamp: eventTime,
				Type:      TypeNodePressure,
				Namespace: "",
				PodName:   "", // Affects all pods on this node
				Message:   msg,
				Source:    w.nodeName,
			}
		}
	}
}
