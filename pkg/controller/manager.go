package controller

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/Dasmat13/kubecorrelate/pkg/printer"
	"github.com/Dasmat13/kubecorrelate/pkg/watcher"
)

type Config struct {
	Namespace     string
	LabelSelector string
	PodRegex      string
	Since         time.Duration
}

type Manager struct {
	client        kubernetes.Interface
	restConfig    *rest.Config
	config        Config
	eventChan     chan watcher.TelemetryEvent
	printer       *printer.ConsolePrinter
	warnedMu      sync.Mutex
	warnedKeys    map[string]bool
	activePodsMu  sync.Mutex
	activePods    map[string]context.CancelFunc
	activeNodes   map[string]bool
	activeConfigs map[string]bool
}

func NewManager(client kubernetes.Interface, restConfig *rest.Config, config Config) *Manager {
	eventChan := make(chan watcher.TelemetryEvent, 1000)
	return &Manager{
		client:        client,
		restConfig:    restConfig,
		config:        config,
		eventChan:     eventChan,
		printer:       printer.NewConsolePrinter(eventChan),
		warnedKeys:    make(map[string]bool),
		activePods:    make(map[string]context.CancelFunc),
		activeNodes:   make(map[string]bool),
		activeConfigs: make(map[string]bool),
	}
}

func (m *Manager) warnOnce(key string, format string, args ...interface{}) {
	m.warnedMu.Lock()
	defer m.warnedMu.Unlock()
	if !m.warnedKeys[key] {
		m.warnedKeys[key] = true
		fmt.Printf("⚠️  "+format+"\n", args...)
	}
}

func (m *Manager) Start(ctx context.Context) error {
	var podRegexCompiled *regexp.Regexp
	if m.config.PodRegex != "" {
		var err error
		podRegexCompiled, err = regexp.Compile(m.config.PodRegex)
		if err != nil {
			return fmt.Errorf("invalid pod name regex: %w", err)
		}
	}

	// Start printer
	go m.printer.Start(ctx)

	listOptions := metav1.ListOptions{}
	if m.config.LabelSelector != "" {
		if _, err := labels.Parse(m.config.LabelSelector); err != nil {
			return fmt.Errorf("invalid label selector: %w", err)
		}
		listOptions.LabelSelector = m.config.LabelSelector
	}

	fmt.Printf("Watching pods in namespace %q matching selector %q...\n", m.config.Namespace, m.config.LabelSelector)
	podWatcher, err := m.client.CoreV1().Pods(m.config.Namespace).Watch(ctx, listOptions)
	if err != nil {
		return fmt.Errorf("failed to start pod watch: %w", err)
	}
	defer podWatcher.Stop()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Monitoring Loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-podWatcher.ResultChan():
				if !ok {
					// Watcher channel closed unexpectedly, attempt restart
					var restartErr error
					podWatcher, restartErr = m.client.CoreV1().Pods(m.config.Namespace).Watch(ctx, listOptions)
					if restartErr != nil {
						m.warnOnce("pod-watch-restart", "Failed to restart pod watcher: %v", restartErr)
						time.Sleep(2 * time.Second)
					}
					continue
				}

				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}

				// Apply regex filtering if configured
				if podRegexCompiled != nil && !podRegexCompiled.MatchString(pod.Name) {
					continue
				}

				switch event.Type {
				case watch.Added, watch.Modified:
					m.activePodsMu.Lock()
					_, active := m.activePods[pod.Name]
					if !active && pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
						podCtx, cancelPod := context.WithCancel(ctx)
						m.activePods[pod.Name] = cancelPod

						wg.Add(1)
						go func(p corev1.Pod) {
							defer wg.Done()
							m.startWatchersForPod(podCtx, p)
						}(*pod)
					}
					m.activePodsMu.Unlock()

				case watch.Deleted:
					m.activePodsMu.Lock()
					cancelPod, active := m.activePods[pod.Name]
					if active {
						cancelPod()
						delete(m.activePods, pod.Name)
					}
					m.activePodsMu.Unlock()
				}
			}
		}
	}()

	// Wait for context termination
	<-ctx.Done()

	// Cancel all active pod watcher contexts
	m.activePodsMu.Lock()
	for _, cancelPod := range m.activePods {
		cancelPod()
	}
	m.activePodsMu.Unlock()

	// Wait for all watcher goroutines to exit cleanly
	wg.Wait()
	close(m.eventChan)

	return nil
}

func (m *Manager) startWatchersForPod(ctx context.Context, pod corev1.Pod) {
	var wg sync.WaitGroup

	// 1. Log Watcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		logWatcher := watcher.NewLogWatcher(m.client, pod.Namespace, pod.Name, m.config.Since)
		if err := logWatcher.Watch(ctx, m.eventChan); err != nil {
			if apierrors.IsForbidden(err) {
				m.warnOnce("logs-"+pod.Namespace, "Log monitoring disabled in namespace %q due to insufficient RBAC permissions", pod.Namespace)
				return
			}
			m.eventChan <- watcher.TelemetryEvent{
				Timestamp: time.Now(),
				Type:      watcher.TypeEvent,
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				Message:   fmt.Sprintf("Log watcher failed: %v", err),
				Source:    "KubeCorrelate",
			}
		}
	}()

	// 2. Pod Event Watcher
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventWatcher := watcher.NewEventWatcher(m.client, pod.Namespace, pod.Name)
		if err := eventWatcher.Watch(ctx, m.eventChan); err != nil {
			if apierrors.IsForbidden(err) {
				m.warnOnce("events-"+pod.Namespace, "Pod event monitoring disabled in namespace %q due to insufficient RBAC permissions", pod.Namespace)
				return
			}
			m.eventChan <- watcher.TelemetryEvent{
				Timestamp: time.Now(),
				Type:      watcher.TypeEvent,
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				Message:   fmt.Sprintf("Event watcher failed: %v", err),
				Source:    "KubeCorrelate",
			}
		}
	}()

	// 3. Node Health Watcher
	if pod.Spec.NodeName != "" {
		var registerNode bool
		m.activePodsMu.Lock()
		if !m.activeNodes[pod.Spec.NodeName] {
			m.activeNodes[pod.Spec.NodeName] = true
			registerNode = true
		}
		m.activePodsMu.Unlock()

		if registerNode {
			wg.Add(1)
			go func(nodeName string) {
				defer wg.Done()
				nodeWatcher := watcher.NewNodeWatcher(m.client, nodeName)
				if err := nodeWatcher.Watch(ctx, m.eventChan); err != nil {
					if apierrors.IsForbidden(err) {
						m.warnOnce("nodes", "Node pressure monitoring disabled due to insufficient cluster-wide RBAC permissions")
						return
					}
					m.eventChan <- watcher.TelemetryEvent{
						Timestamp: time.Now(),
						Type:      watcher.TypeNodePressure,
						Namespace: pod.Namespace,
						PodName:   pod.Name,
						Message:   fmt.Sprintf("Node watcher for %s failed: %v", nodeName, err),
						Source:    nodeName,
					}
				}
			}(pod.Spec.NodeName)
		}
	}

	// 4. ConfigMap & Secret Change Watchers
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			cmKey := fmt.Sprintf("%s/cm/%s", pod.Namespace, volume.ConfigMap.Name)
			m.activePodsMu.Lock()
			register := !m.activeConfigs[cmKey]
			if register {
				m.activeConfigs[cmKey] = true
			}
			m.activePodsMu.Unlock()

			if register {
				wg.Add(1)
				go func(cmName string) {
					defer wg.Done()
					configWatcher := watcher.NewConfigWatcher(m.client, pod.Namespace, cmName, watcher.ConfigTypeConfigMap)
					err := configWatcher.Watch(ctx, m.eventChan)
					if err != nil && apierrors.IsForbidden(err) {
						m.warnOnce("configmaps-"+pod.Namespace, "ConfigMap change tracking disabled in namespace %q due to insufficient RBAC permissions", pod.Namespace)
					}
				}(volume.ConfigMap.Name)
			}
		}
		if volume.Secret != nil {
			secKey := fmt.Sprintf("%s/sec/%s", pod.Namespace, volume.Secret.SecretName)
			m.activePodsMu.Lock()
			register := !m.activeConfigs[secKey]
			if register {
				m.activeConfigs[secKey] = true
			}
			m.activePodsMu.Unlock()

			if register {
				wg.Add(1)
				go func(secretName string) {
					defer wg.Done()
					configWatcher := watcher.NewConfigWatcher(m.client, pod.Namespace, secretName, watcher.ConfigTypeSecret)
					err := configWatcher.Watch(ctx, m.eventChan)
					if err != nil && apierrors.IsForbidden(err) {
						m.warnOnce("secrets-"+pod.Namespace, "Secret change tracking disabled in namespace %q due to insufficient RBAC permissions", pod.Namespace)
					}
				}(volume.Secret.SecretName)
			}
		}
	}

	wg.Wait()
}


