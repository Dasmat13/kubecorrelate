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

type ConfigWatcher struct {
	client    kubernetes.Interface
	namespace string
	name      string
	cfgType   ConfigType
}

func NewConfigWatcher(client kubernetes.Interface, namespace, name string, cfgType ConfigType) *ConfigWatcher {
	return &ConfigWatcher{
		client:    client,
		namespace: namespace,
		name:      name,
		cfgType:   cfgType,
	}
}

func (w *ConfigWatcher) Watch(ctx context.Context, eventChan chan<- TelemetryEvent) error {
	opts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", w.name),
		Watch:         true,
	}

	var watcher watch.Interface
	var err error

	if w.cfgType == ConfigTypeConfigMap {
		watcher, err = w.client.CoreV1().ConfigMaps(w.namespace).Watch(ctx, opts)
	} else {
		watcher, err = w.client.CoreV1().Secrets(w.namespace).Watch(ctx, opts)
	}

	if err != nil {
		return err
	}
	defer watcher.Stop()

	// Track version to avoid logging initial connect/list event
	lastResourceVersion := ""

	for {
		select {
		case <-ctx.Done():
			return nil
		case eventObj, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("config watcher closed unexpectedly")
			}

			if eventObj.Type == watch.Error {
				continue
			}

			var version string
			var name string

			if w.cfgType == ConfigTypeConfigMap {
				cm, ok := eventObj.Object.(*corev1.ConfigMap)
				if !ok {
					continue
				}
				version = cm.ResourceVersion
				name = cm.Name
			} else {
				sec, ok := eventObj.Object.(*corev1.Secret)
				if !ok {
					continue
				}
				version = sec.ResourceVersion
				name = sec.Name
			}

			// Skip the initial event when starting the watch
			if lastResourceVersion == "" {
				lastResourceVersion = version
				continue
			}

			if lastResourceVersion == version {
				continue
			}
			lastResourceVersion = version

			if eventObj.Type == watch.Modified {
				eventChan <- TelemetryEvent{
					Timestamp: time.Now(),
					Type:      TypeConfigChange,
					Namespace: w.namespace,
					PodName:   "", // Global to target pods using it
					Message:   fmt.Sprintf("%s %q was modified (resourceVersion: %s)", w.cfgType, name, version),
					Source:    name,
				}
			}
		}
	}
}
