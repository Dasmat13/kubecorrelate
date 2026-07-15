package watcher

import (
	"testing"
	"time"
)

func TestParseLogLine(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		wantMsg       string
		wantTimestamp bool
		timestampStr  string
	}{
		{
			name:          "Valid RFC3339 timestamp",
			line:          "2026-07-15T18:10:29.123456789Z INFO: active connections",
			wantMsg:       "INFO: active connections",
			wantTimestamp: true,
			timestampStr:  "2026-07-15T18:10:29.123456789Z",
		},
		{
			name:          "No timestamp",
			line:          "INFO: heartbeat message",
			wantMsg:       "INFO: heartbeat message",
			wantTimestamp: false,
		},
		{
			name:          "Empty line",
			line:          "",
			wantMsg:       "",
			wantTimestamp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTime, gotMsg := ParseLogLine(tt.line)

			if gotMsg != tt.wantMsg {
				t.Errorf("ParseLogLine() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}

			if tt.wantTimestamp {
				wantTime, err := time.Parse(time.RFC3339Nano, tt.timestampStr)
				if err != nil {
					t.Fatalf("failed to parse wantTimestamp test pattern: %v", err)
				}
				if !gotTime.Equal(wantTime) {
					t.Errorf("ParseLogLine() gotTime = %v, want %v", gotTime, wantTime)
				}
			} else {
				// Should fall back to time.Now() (approximate match)
				if time.Since(gotTime) > 1*time.Second {
					t.Errorf("ParseLogLine() fallback time is too old: %v", gotTime)
				}
			}
		})
	}
}
