package watcher

import (
	"testing"
)

func TestFormatEventMessage(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		reason   string
		message  string
		action   string
		want     string
	}{
		{
			name:     "Message present",
			severity: "INFO",
			reason:   "ScalingReplicaSet",
			message:  "Scaled up replica set nginx-5869d7778c from 0 to 1",
			action:   "ScaleUp",
			want:     "[INFO] ScalingReplicaSet: Scaled up replica set nginx-5869d7778c from 0 to 1",
		},
		{
			name:     "Message empty, Action present",
			severity: "INFO",
			reason:   "Starting",
			message:  "",
			action:   "StartKubeProxy",
			want:     "[INFO] Starting: StartKubeProxy",
		},
		{
			name:     "Message and Action both empty",
			severity: "INFO",
			reason:   "Starting",
			message:  "",
			action:   "",
			want:     "[INFO] Starting",
		},
		{
			name:     "Warning severity, Message present",
			severity: "WARN",
			reason:   "FailedMount",
			message:  "MountVolume.SetUp failed for volume",
			action:   "",
			want:     "[WARN] FailedMount: MountVolume.SetUp failed for volume",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatEventMessage(tt.severity, tt.reason, tt.message, tt.action)
			if got != tt.want {
				t.Errorf("FormatEventMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
