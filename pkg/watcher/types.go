package watcher

import "time"

type TelemetryType string

const (
	TypeLog          TelemetryType = "LOG"
	TypeEvent        TelemetryType = "EVENT"
	TypeConfigChange TelemetryType = "CONFIG"
	TypeNodePressure TelemetryType = "NODE"
)

type TelemetryEvent struct {
	Timestamp time.Time
	Type      TelemetryType
	Namespace string
	PodName   string
	Message   string
	Source    string // Container name for logs, Node name for nodes, Config name for configs
}

type ConfigType string

const (
	ConfigTypeConfigMap ConfigType = "ConfigMap"
	ConfigTypeSecret    ConfigType = "Secret"
)
