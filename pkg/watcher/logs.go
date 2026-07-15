package watcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type LogWatcher struct {
	client    kubernetes.Interface
	namespace string
	podName   string
	since     time.Duration
	filter    string
}

func NewLogWatcher(client kubernetes.Interface, namespace, podName string, since time.Duration, filter string) *LogWatcher {
	return &LogWatcher{
		client:    client,
		namespace: namespace,
		podName:   podName,
		since:     since,
		filter:    filter,
	}
}

func (w *LogWatcher) Watch(ctx context.Context, eventChan chan<- TelemetryEvent) error {
	pod, err := w.client.CoreV1().Pods(w.namespace).Get(ctx, w.podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	sinceTime := metav1.NewTime(time.Now().Add(-w.since))

	// Stream all active and init containers
	containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)

	for _, container := range containers {
		cName := container.Name
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.streamContainerLogs(ctx, cName, &sinceTime, eventChan)
		}()
	}

	wg.Wait()
	return nil
}

func (w *LogWatcher) streamContainerLogs(ctx context.Context, containerName string, sinceTime *metav1.Time, eventChan chan<- TelemetryEvent) {
	opts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		SinceTime:  sinceTime,
		Timestamps: true, // Force-enable API timestamps
	}

	// Retry connection if pod is still initializing
	var stream io.ReadCloser
	var err error

	for i := 0; i < 10; i++ {
		req := w.client.CoreV1().Pods(w.namespace).GetLogs(w.podName, opts)
		stream, err = req.Stream(ctx)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}

	if err != nil {
		eventChan <- TelemetryEvent{
			Timestamp: time.Now(),
			Type:      TypeEvent,
			Namespace: w.namespace,
			PodName:   w.podName,
			Message:   fmt.Sprintf("Failed to stream logs for container %s: %v", containerName, err),
			Source:    containerName,
		}
		return
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					eventChan <- TelemetryEvent{
						Timestamp: time.Now(),
						Type:      TypeEvent,
						Namespace: w.namespace,
						PodName:   w.podName,
						Message:   fmt.Sprintf("Error reading logs from %s: %v", containerName, err),
						Source:    containerName,
					}
				}
				return
			}

			// Clean trailing newline
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}

			eventTimestamp, logMessage := ParseLogLine(line)

			if w.filter != "" {
				if !strings.Contains(strings.ToLower(logMessage), strings.ToLower(w.filter)) {
					continue
				}
			}

			eventChan <- TelemetryEvent{
				Timestamp: eventTimestamp,
				Type:      TypeLog,
				Namespace: w.namespace,
				PodName:   w.podName,
				Message:   logMessage,
				Source:    containerName,
			}
		}
	}
}

func ParseLogLine(line string) (time.Time, string) {
	eventTimestamp := time.Now()
	logMessage := line
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 2 {
		t, err := time.Parse(time.RFC3339Nano, parts[0])
		if err == nil {
			eventTimestamp = t
			logMessage = parts[1]
		}
	}
	return eventTimestamp, logMessage
}


