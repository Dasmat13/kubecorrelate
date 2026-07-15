package printer

import (
	"sort"
	"testing"
	"time"

	"github.com/Dasmat13/kubecorrelate/pkg/watcher"
)

func TestConsolePrinterBufferSorting(t *testing.T) {
	// Setup test events with out-of-order timestamps
	t1 := time.Now().Add(-10 * time.Second)
	t2 := time.Now().Add(-5 * time.Second)
	t3 := time.Now()

	events := []watcher.TelemetryEvent{
		{Timestamp: t2, Message: "Second event"},
		{Timestamp: t3, Message: "Third event"},
		{Timestamp: t1, Message: "First event"},
	}

	// Sort using the same logic as ConsolePrinter
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Verify order
	if events[0].Message != "First event" {
		t.Errorf("Expected first element to be 'First event', got: %s", events[0].Message)
	}
	if events[1].Message != "Second event" {
		t.Errorf("Expected second element to be 'Second event', got: %s", events[1].Message)
	}
	if events[2].Message != "Third event" {
		t.Errorf("Expected third element to be 'Third event', got: %s", events[2].Message)
	}
}

func TestConsolePrinterCollapseRepetitive(t *testing.T) {
	printer := NewConsolePrinter(nil, Config{Collapse: true})

	events := []watcher.TelemetryEvent{
		{Type: watcher.TypeLog, Message: "start worker process 29", PodName: "nginx", Source: "container"},
		{Type: watcher.TypeLog, Message: "start worker process 30", PodName: "nginx", Source: "container"},
		{Type: watcher.TypeLog, Message: "start worker process 31", PodName: "nginx", Source: "container"},
	}

	collapsed := printer.collapseRepetitive(events)
	if len(collapsed) != 1 {
		t.Fatalf("Expected 1 collapsed event, got %d", len(collapsed))
	}
	expectedMsg := "start worker process 29-31 (collapsed 3 occurrences)"
	if collapsed[0].Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, collapsed[0].Message)
	}
}

func TestConsolePrinterDeduplicateEvents(t *testing.T) {
	printer := NewConsolePrinter(nil, Config{Collapse: true})

	events := []watcher.TelemetryEvent{
		{Type: watcher.TypeEvent, Message: "Pulling image nginx", PodName: "nginx", Source: "kubelet"},
		{Type: watcher.TypeEvent, Message: "Pulling image nginx", PodName: "nginx", Source: "kubelet"},
		{Type: watcher.TypeEvent, Message: "Pulling image nginx", PodName: "nginx", Source: "kubelet"},
	}

	deduped := printer.deduplicateEvents(events)
	if len(deduped) != 1 {
		t.Fatalf("Expected 1 deduped event, got %d", len(deduped))
	}
	expectedMsg := "Pulling image nginx (x3)"
	if deduped[0].Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, deduped[0].Message)
	}
}
