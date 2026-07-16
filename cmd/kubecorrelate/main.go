package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/Dasmat13/kubecorrelate/pkg/controller"
)

type ConfigFile struct {
	Namespace     *string `yaml:"namespace,omitempty"`
	LabelSelector *string `yaml:"labelSelector,omitempty"`
	PodRegex      *string `yaml:"podRegex,omitempty"`
	Since         *string `yaml:"since,omitempty"`
	BufferDelay   *string `yaml:"bufferDelay,omitempty"`
	LogFilter     *string `yaml:"logFilter,omitempty"`
	AllNamespaces *bool   `yaml:"allNamespaces,omitempty"`
	RelativeTime  *bool   `yaml:"relativeTime,omitempty"`
	ErrorsOnly    *bool   `yaml:"errorsOnly,omitempty"`
	Timeline      *bool   `yaml:"timeline,omitempty"`
	Collapse      *bool   `yaml:"collapse,omitempty"`
	PodSummary    *bool   `yaml:"podSummary,omitempty"`
}

func loadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	var (
		kubeconfig     string
		namespace      string
		labelSelector  string
		podRegex       string
		sinceStr       string
		bufferDelayStr string
		filterStr      string
		configPath     string
		allNamespaces  bool
		relativeTime   bool
		errorsOnly     bool
		timeline       bool
		collapse       bool
		podSummary     bool
	)

	// Determine default kubeconfig path
	defaultKubeconfig := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKubeconfig = filepath.Join(home, ".kube", "config")
	}

	flag.StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig, "absolute path to the kubeconfig file")
	flag.StringVar(&namespace, "n", "default", "kubernetes namespace to monitor")
	flag.StringVar(&labelSelector, "l", "", "label selector to filter pods (e.g. app=my-app)")
	flag.StringVar(&podRegex, "p", "", "regex pattern to filter pod names")
	flag.StringVar(&sinceStr, "since", "10m", "stream logs since this duration (e.g. 5m, 1h)")
	flag.StringVar(&bufferDelayStr, "buffer-delay", "1.5s", "chronological sorting buffer delay (e.g. 1s, 1.5s, 3s)")
	flag.StringVar(&filterStr, "filter", "", "case-insensitive log filter substring")
	flag.StringVar(&filterStr, "f", "", "case-insensitive log filter substring (shorthand)")
	flag.StringVar(&configPath, "config", "", "path to the config file (yaml)")
	flag.StringVar(&configPath, "c", "", "path to the config file (yaml) (shorthand)")
	flag.BoolVar(&allNamespaces, "A", false, "monitor all namespaces")
	flag.BoolVar(&relativeTime, "relative-time", false, "show relative timestamps instead of absolute time")
	flag.BoolVar(&errorsOnly, "errors-only", false, "display only warning and failure events")
	flag.BoolVar(&timeline, "timeline", false, "suppress verbose application logs and focus on major lifecycle events")
	flag.BoolVar(&collapse, "collapse", true, "collapse repetitive log lines and deduplicate repeated events")
	flag.BoolVar(&podSummary, "pod-summary", true, "print pod lifecycle summary checklist on pod termination")

	flag.Parse()

	// Track which flags were explicitly set by the user
	explicitlySet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitlySet[f.Name] = true
	})

	// Load config file if it exists
	var configFile *ConfigFile
	if configPath != "" {
		var err error
		configFile, err = loadConfigFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read config file %s: %v\n", configPath, err)
			os.Exit(1)
		}
	} else {
		// Auto-detect in current directory
		if _, err := os.Stat(".kubecorrelate.yaml"); err == nil {
			configFile, _ = loadConfigFile(".kubecorrelate.yaml")
		} else if _, err := os.Stat(".kubecorrelate.yml"); err == nil {
			configFile, _ = loadConfigFile(".kubecorrelate.yml")
		}
	}

	if configFile != nil {
		if configFile.Namespace != nil && !explicitlySet["n"] {
			namespace = *configFile.Namespace
		}
		if configFile.LabelSelector != nil && !explicitlySet["l"] {
			labelSelector = *configFile.LabelSelector
		}
		if configFile.PodRegex != nil && !explicitlySet["p"] {
			podRegex = *configFile.PodRegex
		}
		if configFile.Since != nil && !explicitlySet["since"] {
			sinceStr = *configFile.Since
		}
		if configFile.BufferDelay != nil && !explicitlySet["buffer-delay"] {
			bufferDelayStr = *configFile.BufferDelay
		}
		if configFile.LogFilter != nil && !explicitlySet["filter"] && !explicitlySet["f"] {
			filterStr = *configFile.LogFilter
		}
		if configFile.AllNamespaces != nil && !explicitlySet["A"] {
			allNamespaces = *configFile.AllNamespaces
		}
		if configFile.RelativeTime != nil && !explicitlySet["relative-time"] {
			relativeTime = *configFile.RelativeTime
		}
		if configFile.ErrorsOnly != nil && !explicitlySet["errors-only"] {
			errorsOnly = *configFile.ErrorsOnly
		}
		if configFile.Timeline != nil && !explicitlySet["timeline"] {
			timeline = *configFile.Timeline
		}
		if configFile.Collapse != nil && !explicitlySet["collapse"] {
			collapse = *configFile.Collapse
		}
		if configFile.PodSummary != nil && !explicitlySet["pod-summary"] {
			podSummary = *configFile.PodSummary
		}
	}

	// Parse duration
	since, err := time.ParseDuration(sinceStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid duration value for --since: %v\n", err)
		os.Exit(1)
	}

	bufferDelay, err := time.ParseDuration(bufferDelayStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid duration value for --buffer-delay: %v\n", err)
		os.Exit(1)
	}

	// Load kubeconfig config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		// Try in-cluster configuration as fallback
		config, err = clientcmd.BuildConfigFromFlags("", "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load kubeconfig: %v\n", err)
			os.Exit(1)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Override namespace if monitoring all
	if allNamespaces {
		namespace = ""
	}

	// Create context that closes on interrupt
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nStopping KubeCorrelate...")
		cancel()
	}()

	fmt.Println("Starting KubeCorrelate... Press Ctrl+C to stop.")

	// Configure and start manager
	mgr := controller.NewManager(clientset, config, controller.Config{
		Namespace:     namespace,
		LabelSelector: labelSelector,
		PodRegex:      podRegex,
		Since:         since,
		BufferDelay:   bufferDelay,
		LogFilter:     filterStr,
		RelativeTime:  relativeTime,
		ErrorsOnly:    errorsOnly,
		Timeline:      timeline,
		Collapse:      collapse,
		PodSummary:    podSummary,
	})

	if err := mgr.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: manager run failed: %v\n", err)
		os.Exit(1)
	}
}
