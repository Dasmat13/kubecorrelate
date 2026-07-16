package printer

import (
	"fmt"
	"testing"
	"time"

	"github.com/Dasmat13/kubecorrelate/pkg/watcher"
)

func BenchmarkConsolePrinterCollapse(b *testing.B) {
	// Generate events
	events := make([]watcher.TelemetryEvent, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = watcher.TelemetryEvent{
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
			Type:      watcher.TypeLog,
			Namespace: "default",
			PodName:   "test-pod-1",
			Message:   fmt.Sprintf("some log message with count %d", i),
			Source:    "container-1",
		}
	}

	p := &ConsolePrinter{
		config: Config{
			Collapse: true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.collapseRepetitive(events)
	}
}

func BenchmarkConsolePrinterDeduplicate(b *testing.B) {
	// Generate duplicate events
	events := make([]watcher.TelemetryEvent, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = watcher.TelemetryEvent{
			Timestamp: time.Now(),
			Type:      watcher.TypeEvent,
			Namespace: "default",
			PodName:   "test-pod-1",
			Message:   "Back-off restarting failed container",
			Source:    "k8s-event",
		}
	}

	p := &ConsolePrinter{
		config: Config{
			Collapse: true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.deduplicateEvents(events)
	}
}
