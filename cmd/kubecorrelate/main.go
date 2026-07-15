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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/Dasmat13/kubecorrelate/pkg/controller"
)

func main() {
	var (
		kubeconfig     string
		namespace      string
		labelSelector  string
		podRegex       string
		sinceStr       string
		bufferDelayStr string
		allNamespaces  bool
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
	flag.BoolVar(&allNamespaces, "A", false, "monitor all namespaces")

	flag.Parse()

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
	})

	if err := mgr.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: manager run failed: %v\n", err)
		os.Exit(1)
	}
}
