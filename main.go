package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"kblog/k8s"
	"kblog/tui"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var version = "dev"

func main() {
	// Parse CLI flags
	versionFlag := flag.Bool("version", false, "Print version and exit")
	contextFlag := flag.String("context", "", "Kubernetes cluster context name")
	namespaceFlag := flag.String("namespace", "", "Target Kubernetes namespace")
	podFlag := flag.String("pod", "", "Target Pod name to stream logs")
	deploymentFlag := flag.String("deployment", "", "Target Deployment name to stream logs from all replicas")
	tailFlag := flag.Int64("tail", 200, "Number of initial log lines to tail")
	themeFlag := flag.String("theme", "terminal", "Initial color theme (terminal, midnight, dracula, catppuccin, nord, monokai)")

	flag.Parse()

	if *versionFlag {
		fmt.Println("kblog", version)
		os.Exit(0)
	}

	// Set initial color theme
	foundTheme := false
	for _, t := range tui.Themes {
		cleanFlag := strings.ToLower(strings.TrimSpace(*themeFlag))
		cleanName := strings.ToLower(strings.Split(t.Name, " ")[0])
		if cleanFlag == cleanName {
			tui.InitStyles(t)
			foundTheme = true
			break
		}
	}
	if !foundTheme && *themeFlag != "" && *themeFlag != "terminal" {
		fmt.Printf("Warning: Theme %q not found. Defaulting to Terminal.\n", *themeFlag)
	}

	// Validation
	if *podFlag == "" && *deploymentFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: You must specify either a --pod or a --deployment to view logs.")
		flag.Usage()
		os.Exit(1)
	}

	// Suppress status lines when running interactively (TUI handles visuals)
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	// Load Kubernetes API Client
	if !isTTY {
		fmt.Printf("Connecting to Kubernetes cluster (Context: %q, Namespace: %q)...\n", *contextFlag, *namespaceFlag)
	}
	clientInfo, err := k8s.LoadClient(*contextFlag, *namespaceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to connect to Kubernetes: %v\n", err)
		os.Exit(1)
	}

	ns := clientInfo.Namespace
	clientset := clientInfo.Clientset

	var selectorStr string
	var targetPods []string

	// If deployment is targeted, find its pods via selector
	if *deploymentFlag != "" {
		if !isTTY {
			fmt.Printf("Fetching Deployment %q in namespace %q...\n", *deploymentFlag, ns)
		}
		deploy, err := clientset.AppsV1().Deployments(ns).Get(context.Background(), *deploymentFlag, metav1.GetOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to fetch Deployment: %v\n", err)
			os.Exit(1)
		}

		if deploy.Spec.Selector == nil {
			fmt.Fprintln(os.Stderr, "Error: Selected deployment does not define a label selector.")
			os.Exit(1)
		}

		// Convert K8s LabelSelector to a string selector, e.g. "app=auth-service"
		labelSelector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid deployment label selector: %v\n", err)
			os.Exit(1)
		}
		selectorStr = labelSelector.String()

		// Fetch current active pods for initial event watcher targets
		podList, err := clientset.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{
			LabelSelector: selectorStr,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to fetch active pods for event tracking: %v. Event notifications may be incomplete.\n", err)
		} else {
			for _, p := range podList.Items {
				targetPods = append(targetPods, p.Name)
			}
		}
	}

	// Create root cancelable context for background watches
	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sharedLogChan := make(chan k8s.LogLine, 2000)

	// Initialize log streamer — writes directly to sharedLogChan
	var streamer *k8s.LogStreamer
	if *podFlag != "" {
		streamer = k8s.NewLogStreamer(clientset, ns, *podFlag, "", *tailFlag, sharedLogChan)
		targetPods = []string{*podFlag}
	} else {
		streamer = k8s.NewLogStreamer(clientset, ns, "", selectorStr, *tailFlag, sharedLogChan)
	}

	if err := streamer.Start(bgCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to start log streamer: %v\n", err)
		os.Exit(1)
	}

	// Initialize K8s event watcher — also writes directly to sharedLogChan
	var watcher *k8s.EventWatcher
	if *podFlag != "" {
		watcher = k8s.NewEventWatcher(clientset, ns, *podFlag, sharedLogChan)
	} else {
		watcher = k8s.NewEventWatcher(clientset, ns, "", sharedLogChan)
		watcher.UpdateTargets(targetPods)
	}

	// In deployment mode, keep the event watcher's target pod set in sync as the
	// streamer re-discovers pods (scale-ups, rolling updates).
	if *deploymentFlag != "" {
		streamer.SetOnPodsChanged(func(pods []string) {
			watcher.UpdateTargets(pods)
		})
	}

	if err := watcher.Start(bgCtx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to start event watcher: %v\n", err)
		os.Exit(1)
	}

	// Close sharedLogChan once both producers have stopped writing.
	go func() {
		<-streamer.Done()
		watcher.Stop() // waits for watcher goroutine to exit before we close
		close(sharedLogChan)
	}()

	// Initialize bubbletea model
	ctxDisplayName := *contextFlag
	if ctxDisplayName == "" {
		ctxDisplayName = "current"
	}
	model := tui.NewModel(ctxDisplayName, ns, *podFlag, *deploymentFlag, version, sharedLogChan, streamer, watcher, cancel)

	// Run bubbletea AltScreen program (takes over terminal and restores on exit)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running kblog TUI: %v\n", err)
		os.Exit(1)
	}
}
