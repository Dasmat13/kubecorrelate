package printer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Dasmat13/kubecorrelate/pkg/watcher"
)

const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32" + "m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
	colorBold    = "\033[1m"

	flushTick = 100 * time.Millisecond
)

type Config struct {
	BufferDelay  time.Duration
	RelativeTime bool
	ErrorsOnly   bool
	Timeline     bool
	Collapse     bool
	PodSummary   bool
}

type PodHistory struct {
	StartTime time.Time
	EndTime   time.Time
	Events    []string
}

type ConsolePrinter struct {
	eventChan      <-chan watcher.TelemetryEvent
	buffer         []watcher.TelemetryEvent
	mu             sync.Mutex
	config         Config
	startTime      time.Time
	lastTime       time.Time
	lastTimeMu     sync.Mutex
	podHistories   map[string]*PodHistory
	podHistoriesMu sync.Mutex
}

func NewConsolePrinter(eventChan <-chan watcher.TelemetryEvent, config Config) *ConsolePrinter {
	return &ConsolePrinter{
		eventChan:    eventChan,
		buffer:       make([]watcher.TelemetryEvent, 0),
		config:       config,
		podHistories: make(map[string]*PodHistory),
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
	matureThreshold := now.Add(-p.config.BufferDelay)

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

		if p.config.Collapse {
			mature = p.collapseRepetitive(mature)
			mature = p.deduplicateEvents(mature)
		}

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

	events := p.buffer
	if p.config.Collapse {
		events = p.collapseRepetitive(events)
		events = p.deduplicateEvents(events)
	}

	for _, event := range events {
		p.printEvent(event)
	}

	p.buffer = nil
}

var numRegex = regexp.MustCompile(`\d+`)

func extractPattern(msg string) (string, int) {
	locs := numRegex.FindAllStringIndex(msg, -1)
	if len(locs) == 0 {
		return msg, -1
	}
	lastLoc := locs[len(locs)-1]
	numberStr := msg[lastLoc[0]:lastLoc[1]]
	var val int
	fmt.Sscanf(numberStr, "%d", &val)
	pattern := msg[:lastLoc[0]] + "%d" + msg[lastLoc[1]:]
	return pattern, val
}

func (p *ConsolePrinter) collapseRepetitive(events []watcher.TelemetryEvent) []watcher.TelemetryEvent {
	if len(events) < 2 {
		return events
	}

	var collapsed []watcher.TelemetryEvent
	i := 0
	for i < len(events) {
		event := events[i]
		if event.Type != watcher.TypeLog {
			collapsed = append(collapsed, event)
			i++
			continue
		}

		pattern, val := extractPattern(event.Message)
		if val == -1 {
			collapsed = append(collapsed, event)
			i++
			continue
		}

		j := i + 1
		var vals []int
		vals = append(vals, val)
		for j < len(events) &&
			events[j].Type == event.Type &&
			events[j].PodName == event.PodName &&
			events[j].Source == event.Source {
			p2, v2 := extractPattern(events[j].Message)
			if p2 == pattern && v2 != -1 {
				vals = append(vals, v2)
				j++
			} else {
				break
			}
		}

		if len(vals) >= 3 {
			minVal := vals[0]
			maxVal := vals[0]
			for _, v := range vals {
				if v < minVal {
					minVal = v
				}
				if v > maxVal {
					maxVal = v
				}
			}

			cleanPattern := strings.Replace(pattern, "%d", fmt.Sprintf("%d-%d", minVal, maxVal), 1)
			summaryMsg := fmt.Sprintf("%s (collapsed %d occurrences)", cleanPattern, len(vals))

			collapsedEvent := event
			collapsedEvent.Message = summaryMsg
			collapsedEvent.Timestamp = events[j-1].Timestamp
			collapsed = append(collapsed, collapsedEvent)
			i = j
		} else {
			for k := i; k < j; k++ {
				collapsed = append(collapsed, events[k])
			}
			i = j
		}
	}
	return collapsed
}

func (p *ConsolePrinter) deduplicateEvents(events []watcher.TelemetryEvent) []watcher.TelemetryEvent {
	if len(events) < 2 {
		return events
	}
	var deduped []watcher.TelemetryEvent
	i := 0
	for i < len(events) {
		event := events[i]
		if event.Type != watcher.TypeEvent && event.Type != watcher.TypeNodePressure {
			deduped = append(deduped, event)
			i++
			continue
		}

		j := i + 1
		for j < len(events) &&
			events[j].Type == event.Type &&
			events[j].PodName == event.PodName &&
			events[j].Source == event.Source &&
			events[j].Message == event.Message {
			j++
		}

		count := j - i
		if count > 1 {
			dedupedEvent := event
			dedupedEvent.Message = fmt.Sprintf("%s (x%d)", event.Message, count)
			dedupedEvent.Timestamp = events[j-1].Timestamp
			deduped = append(deduped, dedupedEvent)
		} else {
			deduped = append(deduped, event)
		}
		i = j
	}
	return deduped
}

func (p *ConsolePrinter) printEvent(event watcher.TelemetryEvent) {
	// Errors only viewing mode
	if p.config.ErrorsOnly {
		isError := false
		if event.Type == watcher.TypeNodePressure {
			isError = !strings.Contains(event.Message, "[INFO]")
		} else if event.Type == watcher.TypeEvent {
			isError = strings.Contains(event.Message, "[WARN]") ||
				strings.Contains(event.Message, "Failed") ||
				strings.Contains(event.Message, "BackOff") ||
				strings.Contains(event.Message, "OOM") ||
				strings.Contains(event.Message, "Unhealthy") ||
				strings.Contains(event.Message, "Error")
		}
		if !isError {
			return
		}
	}

	// Timeline only viewing mode
	if p.config.Timeline && event.Type == watcher.TypeLog {
		return
	}

	// Pod Lifecycle History Collection
	if event.PodName != "" {
		p.podHistoriesMu.Lock()
		hist, exists := p.podHistories[event.PodName]
		if !exists {
			hist = &PodHistory{StartTime: event.Timestamp}
			p.podHistories[event.PodName] = hist
		}
		if event.Type == watcher.TypeEvent {
			hist.Events = append(hist.Events, event.Message)
		}
		hist.EndTime = event.Timestamp
		p.podHistoriesMu.Unlock()
	}

	// Group Deployment Lifecycle Events
	if strings.Contains(event.Message, "🚀 Rollout detected: Scaled up") {
		fmt.Printf("\n%s━━━━━━━━━━━━ Rollout Started ━━━━━━━━━━━━%s\n\n", colorBold+colorMagenta, colorReset)
	} else if strings.Contains(event.Message, "🚀 Rollout detected: Scaled down") {
		fmt.Printf("\n%s━━━━━━━━━━━━ Old Pod Shutdown ━━━━━━━━━━━━%s\n\n", colorBold+colorMagenta, colorReset)
	}

	// Optional Relative Timestamps
	var timestampStr string
	if p.config.RelativeTime {
		p.lastTimeMu.Lock()
		if p.startTime.IsZero() {
			p.startTime = event.Timestamp
			p.lastTime = event.Timestamp
		}
		durationSincePrev := event.Timestamp.Sub(p.lastTime)
		p.lastTime = event.Timestamp
		p.lastTimeMu.Unlock()

		if durationSincePrev < time.Second {
			timestampStr = fmt.Sprintf("+%dms", durationSincePrev.Milliseconds())
		} else {
			timestampStr = fmt.Sprintf("+%.1fs", durationSincePrev.Seconds())
		}
	} else {
		timestampStr = event.Timestamp.Local().Format("15:04:05.000")
	}

	// Print formatted events with prefixes and visual highlights
	switch event.Type {
	case watcher.TypeLog:
		fmt.Printf("[%s%s%s] [%s📜 LOG%s] [%s%s%s/%s%s%s] %s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, colorReset,
			colorGreen, event.PodName, colorReset,
			colorCyan, event.Source, colorReset,
			event.Message,
		)

	case watcher.TypeEvent:
		badgeColor := colorBlue
		msgColor := colorReset
		if strings.Contains(event.Message, "[WARN]") {
			badgeColor = colorYellow
			msgColor = colorYellow
		} else if strings.Contains(event.Message, "Failed") ||
			strings.Contains(event.Message, "BackOff") ||
			strings.Contains(event.Message, "OOM") ||
			strings.Contains(event.Message, "Unhealthy") ||
			strings.Contains(event.Message, "Error") {
			badgeColor = colorRed
			msgColor = colorRed
		}
		fmt.Printf("[%s%s%s] [%s%s📅 EVENT%s] [%s%s%s] %s%s%s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, badgeColor, colorReset,
			colorWhite, event.PodName, colorReset,
			msgColor, event.Message, colorReset,
		)

	case watcher.TypeConfigChange:
		fmt.Printf("[%s%s%s] [%s%s⚙️ CONFIG%s] [%s%s%s] %s%s%s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, colorMagenta, colorReset,
			colorWhite, event.Source, colorReset,
			colorMagenta, event.Message, colorReset,
		)

	case watcher.TypeNodePressure:
		badgeColor := colorRed
		msgColor := colorRed
		if strings.Contains(event.Message, "[INFO]") {
			badgeColor = colorBlue
			msgColor = colorReset
		}
		fmt.Printf("[%s%s%s] [%s%s🖥 NODE%s] [%s%s%s] %s%s%s\n",
			colorCyan, timestampStr, colorReset,
			colorBold, badgeColor, colorReset,
			colorWhite, event.Source, colorReset,
			msgColor, event.Message, colorReset,
		)
	}

	// Pod Lifecycle Summary
	if p.config.PodSummary && event.PodName != "" && strings.Contains(event.Message, "Pod terminated/deleted") {
		p.podHistoriesMu.Lock()
		hist, exists := p.podHistories[event.PodName]
		if exists {
			delete(p.podHistories, event.PodName)
			p.podHistoriesMu.Unlock()

			scheduled := false
			pulled := false
			started := false
			terminated := true

			for _, e := range hist.Events {
				if strings.Contains(e, "Scheduled") {
					scheduled = true
				}
				if strings.Contains(e, "Pulled") {
					pulled = true
				}
				if strings.Contains(e, "Started") {
					started = true
				}
			}

			duration := hist.EndTime.Sub(hist.StartTime).Round(time.Second)

			fmt.Printf("\n%s━━━━━━━━━━━━ Pod Lifecycle Summary: %s ━━━━━━━━━━━━%s\n", colorBold, event.PodName, colorReset)
			if scheduled {
				fmt.Printf("  %s✓%s Scheduled\n", colorGreen, colorReset)
			} else {
				fmt.Printf("  %s✗%s Scheduled\n", colorRed, colorReset)
			}
			if pulled {
				fmt.Printf("  %s✓%s Pulled image\n", colorGreen, colorReset)
			} else {
				fmt.Printf("  %s✗%s Pulled image\n", colorRed, colorReset)
			}
			if started {
				fmt.Printf("  %s✓%s Started\n", colorGreen, colorReset)
			} else {
				fmt.Printf("  %s✗%s Started\n", colorRed, colorReset)
			}
			if terminated {
				fmt.Printf("  %s✓%s Terminated\n", colorGreen, colorReset)
			}
			fmt.Printf("\n  Duration: %s\n", duration)
			fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", colorBold, colorReset)
		} else {
			p.podHistoriesMu.Unlock()
		}
	}
}
