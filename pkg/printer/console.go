package printer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Dasmat13/kubecorrelate/pkg/watcher"
)
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorMagenta= "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"

	flushTick   = 100 * time.Millisecond
)

type ConsolePrinter struct {
	eventChan   <-chan watcher.TelemetryEvent
	buffer      []watcher.TelemetryEvent
	mu          sync.Mutex
	bufferDelay time.Duration
}

func NewConsolePrinter(eventChan <-chan watcher.TelemetryEvent, bufferDelay time.Duration) *ConsolePrinter {
	return &ConsolePrinter{
		eventChan:   eventChan,
		buffer:      make([]watcher.TelemetryEvent, 0),
		bufferDelay: bufferDelay,
	}
}

func (p *ConsolePrinter) Start(ctx context.Context) {
	ticker := time.NewTicker(flushTick)
	defer ticker.Stop()

	// Goroutine to consume incoming events into the buffer
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-p.eventChan:
				if !ok {
					return
				}
				p.mu.Lock()
				p.buffer = append(p.buffer, event)
				p.mu.Unlock()
			}
		}
	}()

	// Loop to flush mature events
	for {
		select {
		case <-ctx.Done():
			p.flushAll()
			return
		case <-ticker.C:
			p.flushMature()
		}
	}
}

func (p *ConsolePrinter) flushMature() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.buffer) == 0 {
		return
	}

	now := time.Now()
	matureThreshold := now.Add(-p.bufferDelay)

	var mature []watcher.TelemetryEvent
	var remaining []watcher.TelemetryEvent

	for _, event := range p.buffer {
		if event.Timestamp.Before(matureThreshold) {
			mature = append(mature, event)
		} else {
			remaining = append(remaining, event)
		}
	}

	p.buffer = remaining

	if len(mature) > 0 {
		// Sort chronologically by timestamp
		sort.Slice(mature, func(i, j int) bool {
			return mature[i].Timestamp.Before(mature[j].Timestamp)
		})

		for _, event := range mature {
			p.printEvent(event)
		}
	}
}

func (p *ConsolePrinter) flushAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.buffer) == 0 {
		return
	}

	sort.Slice(p.buffer, func(i, j int) bool {
		return p.buffer[i].Timestamp.Before(p.buffer[j].Timestamp)
	})

	for _, event := range p.buffer {
		p.printEvent(event)
	}

	p.buffer = nil
}

func (p *ConsolePrinter) printEvent(event watcher.TelemetryEvent) {
	timestampStr := event.Timestamp.Local().Format("15:04:05.000")

	switch event.Type {
	case watcher.TypeLog:
		// Log line: Cyan timestamp, green container name, default message
		fmt.Printf("[%s%s%s] [%sLOG%s] [%s%s%s/%s%s%s] %s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, colorReset,
			colorGreen, event.PodName, colorReset,
			colorCyan, event.Source, colorReset,
			event.Message,
		)

	case watcher.TypeEvent:
		// K8s Event: Yellow/Blue timestamp/badge, bold message
		badgeColor := colorBlue
		if strings.Contains(event.Message, "[WARN]") {
			badgeColor = colorYellow
		}
		fmt.Printf("[%s%s%s] [%s%sEVENT%s] [%s%s%s] %s%s%s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, badgeColor, colorReset,
			colorWhite, event.PodName, colorReset,
			badgeColor, event.Message, colorReset,
		)

	case watcher.TypeConfigChange:
		// Config modification: Magenta badge, bold message
		fmt.Printf("[%s%s%s] [%s%sCONFIG%s] [%s%s%s] %s%s%s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, colorMagenta, colorReset,
			colorWhite, event.Source, colorReset,
			colorMagenta, event.Message, colorReset,
		)

	case watcher.TypeNodePressure:
		// Node warning: Red badge, bold message
		fmt.Printf("[%s%s%s] [%s%sNODE%s] [%s%s%s] %s%s%s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, colorRed, colorReset,
			colorWhite, event.Source, colorReset,
			colorRed, event.Message, colorReset,
		)
	}
}
