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

type EventWatcher struct {
	client    kubernetes.Interface
	namespace string
	podName   string
}

func NewEventWatcher(client kubernetes.Interface, namespace, podName string) *EventWatcher {
	return &EventWatcher{
		client:    client,
		namespace: namespace,
		podName:   podName,
	}
}

func (w *EventWatcher) Watch(ctx context.Context, eventChan chan<- TelemetryEvent) error {
	// Only fetch events for this specific pod
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", w.podName)
	opts := metav1.ListOptions{
		FieldSelector: fieldSelector,
		Watch:         true,
	}

	watcher, err := w.client.CoreV1().Events(w.namespace).Watch(ctx, opts)
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
				return fmt.Errorf("event channel closed unexpectedly")
			}

			if eventObj.Type == watch.Error {
				continue
			}

			// Parse Event object
			k8sEvent, ok := eventObj.Object.(*corev1.Event)
			if !ok {
				continue
			}

			// Skip older events (we are interested in current active events)
			eventTime := k8sEvent.LastTimestamp.Time
			if eventTime.IsZero() {
				eventTime = k8sEvent.FirstTimestamp.Time
			}
			if eventTime.IsZero() {
				eventTime = time.Now()
			}

			// If event is more than 1 minute old and we're starting, skip it to avoid spamming the logs
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
				Type:      TypeEvent,
				Namespace: w.namespace,
				PodName:   w.podName,
				Message:   msg,
				Source:    "k8s-event",
			}
		}
	}
}

