package k8s

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LogLine represents a single processed line in the stream
type LogLine struct {
	Timestamp time.Time
	Pod       string
	Container string
	Content   string
	IsEvent   bool
	Level     string            // INFO, WARN, ERROR, DEBUG
	Fields    map[string]string // populated for JSON log lines; nil otherwise
}

// podPollInterval controls how often deployment (selector) mode re-discovers
// pods so that replicas created after startup (scale-ups, rolling updates) are
// picked up.
const podPollInterval = 5 * time.Second

// Reconnect backoff bounds shared by the log streamer and event watcher.
const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// nextBackoff doubles the current backoff, capped at maxBackoff.
func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// LogStreamer handles concurrent log streaming from K8s pods
type LogStreamer struct {
	clientset *kubernetes.Clientset
	namespace string
	podName   string
	selector  string // Label selector for multi-pod tailing
	tailLines int64
	outChan   chan LogLine
	done      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// streaming tracks the pod/container pairs that already have an active
	// streaming goroutine, so the poller doesn't double-stream them.
	streamingMu sync.Mutex
	streaming   map[string]bool

	// onPodsChanged, if set, is invoked with the current set of pod names each
	// time the pod set is (re)discovered in selector mode. Used to keep the
	// event watcher's target set in sync with rolling updates / scaling.
	onPodsChanged func([]string)
}

// NewLogStreamer creates a new streamer for a specific pod or label selector.
// out is the destination channel; the caller owns it and must not close it before Done() fires.
func NewLogStreamer(clientset *kubernetes.Clientset, namespace string, podName string, selector string, tailLines int64, out chan LogLine) *LogStreamer {
	return &LogStreamer{
		clientset: clientset,
		namespace: namespace,
		podName:   podName,
		selector:  selector,
		tailLines: tailLines,
		outChan:   out,
		streaming: make(map[string]bool),
	}
}

// Done returns a channel that is closed when all streaming goroutines have exited.
func (s *LogStreamer) Done() <-chan struct{} { return s.done }

// SetOnPodsChanged registers a callback that receives the current set of pod
// names whenever the streamer (re)discovers pods in selector mode.
func (s *LogStreamer) SetOnPodsChanged(fn func([]string)) {
	s.onPodsChanged = fn
}

// Start starts streaming logs concurrently. Logs are written to the channel passed to NewLogStreamer.
// Done() is closed when all streaming goroutines exit.
func (s *LogStreamer) Start(parentCtx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(parentCtx)
	s.done = make(chan struct{})

	if s.podName != "" {
		pod, err := s.clientset.CoreV1().Pods(s.namespace).Get(s.ctx, s.podName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pod %s: %w", s.podName, err)
		}
		s.spawnStreamsForPods([]corev1.Pod{*pod})
	} else if s.selector != "" {
		podList, err := s.clientset.CoreV1().Pods(s.namespace).List(s.ctx, metav1.ListOptions{
			LabelSelector: s.selector,
		})
		if err != nil {
			return fmt.Errorf("failed to list pods with selector %s: %w", s.selector, err)
		}

		if len(podList.Items) == 0 {
			// No pods yet — keep polling so pods created later still get picked up.
			select {
			case s.outChan <- LogLine{
				Timestamp: time.Now(),
				Content:   "No matching pods found yet. Watching for new pods...",
				IsEvent:   true,
				Level:     "WARN",
			}:
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
		} else {
			s.spawnStreamsForPods(podList.Items)
		}

		s.wg.Add(1)
		go s.watchPodsLoop()
	} else {
		return fmt.Errorf("either podName or selector must be provided")
	}

	go func() {
		s.wg.Wait()
		close(s.done)
	}()

	return nil
}

// podContainer identifies a single streamable container within a pod.
type podContainer struct {
	pod       string
	container string
}

func (pc podContainer) key() string { return pc.pod + "/" + pc.container }

// enumerateContainers returns every streamable container (init + regular)
// across the given pods, preserving pod and within-pod ordering.
func enumerateContainers(pods []corev1.Pod) []podContainer {
	var out []podContainer
	for _, pod := range pods {
		for _, c := range pod.Spec.InitContainers {
			out = append(out, podContainer{pod: pod.Name, container: c.Name})
		}
		for _, c := range pod.Spec.Containers {
			out = append(out, podContainer{pod: pod.Name, container: c.Name})
		}
	}
	return out
}

// podNamesOf returns the names of the given pods.
func podNamesOf(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	return names
}

// claimNewStreams marks the given containers as streaming and returns only
// those that were not already being streamed (i.e. need a fresh goroutine).
// The check-and-set is atomic per container so concurrent callers and the
// stream-exit cleanup never double-stream or miss a container.
func (s *LogStreamer) claimNewStreams(pcs []podContainer) []podContainer {
	var fresh []podContainer
	for _, pc := range pcs {
		s.streamingMu.Lock()
		if s.streaming[pc.key()] {
			s.streamingMu.Unlock()
			continue
		}
		s.streaming[pc.key()] = true
		s.streamingMu.Unlock()
		fresh = append(fresh, pc)
	}
	return fresh
}

// spawnStreamsForPods starts a streaming goroutine for every container in the
// given pods that is not already being streamed. It also notifies any
// registered onPodsChanged callback with the current pod set. Safe for
// concurrent use (invoked from both Start and the poller).
func (s *LogStreamer) spawnStreamsForPods(pods []corev1.Pod) {
	for _, pc := range s.claimNewStreams(enumerateContainers(pods)) {
		s.wg.Add(1)
		go s.streamContainerLogs(pc.pod, pc.container)
	}

	if s.onPodsChanged != nil {
		s.onPodsChanged(podNamesOf(pods))
	}
}

// watchPodsLoop periodically re-lists pods matching the selector and starts
// streaming any newly-appeared replicas. Runs until the context is cancelled.
func (s *LogStreamer) watchPodsLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(podPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			podList, err := s.clientset.CoreV1().Pods(s.namespace).List(s.ctx, metav1.ListOptions{
				LabelSelector: s.selector,
			})
			if err != nil {
				// Transient listing error; try again on the next tick.
				continue
			}
			s.spawnStreamsForPods(podList.Items)
		}
	}
}

// Stop terminates all active log streams
func (s *LogStreamer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// streamContainerLogs tails logs for a specific pod/container
func (s *LogStreamer) streamContainerLogs(podName, containerName string) {
	defer s.wg.Done()
	// Release the streaming slot on exit so a pod that reappears with the same
	// name can be re-streamed by the poller.
	defer func() {
		key := podName + "/" + containerName
		s.streamingMu.Lock()
		delete(s.streaming, key)
		s.streamingMu.Unlock()
	}()

	tail := s.tailLines
	opts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		TailLines:  &tail,
		Timestamps: true, // We want K8s to add RFC3339 timestamps for chronological sorting
	}

	// Retry loop for transient connection dropouts. Uses exponential backoff and
	// collapses a run of consecutive failures into a single WARN line (re-armed
	// only after a successful reconnect) so a permanently-failing container
	// doesn't flood the timeline every few seconds.
	backoff := initialBackoff
	failureNotified := false

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		req := s.clientset.CoreV1().Pods(s.namespace).GetLogs(podName, opts)
		stream, err := req.Stream(s.ctx)
		if err != nil {
			if !failureNotified {
				select {
				case s.outChan <- LogLine{
					Timestamp: time.Now(),
					Pod:       podName,
					Container: containerName,
					Content:   fmt.Sprintf("[System] Failed to stream logs: %v. Retrying with backoff (up to %s)...", err, maxBackoff),
					IsEvent:   true,
					Level:     "WARN",
				}:
				case <-s.ctx.Done():
					return
				}
				failureNotified = true
			}
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(backoff):
				backoff = nextBackoff(backoff)
				continue
			}
		}

		// Connected successfully: reset backoff and re-arm failure notification.
		backoff = initialBackoff
		failureNotified = false

		s.readStream(stream, podName, containerName)
		_ = stream.Close()

		// Stream ended - if context is cancelled, exit. Otherwise, wait and retry.
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(2 * time.Second):
			// For subsequent reconnects, tail only a few lines to avoid duplication
			newTail := int64(10)
			opts.TailLines = &newTail
		}
	}
}

// readStream parses log lines from the stream reader
func (s *LogStreamer) readStream(stream io.ReadCloser, podName, containerName string) {
	reader := bufio.NewReader(stream)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		lineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				select {
				case s.outChan <- LogLine{
					Timestamp: time.Now(),
					Pod:       podName,
					Container: containerName,
					Content:   fmt.Sprintf("[System] Error reading stream: %v", err),
					IsEvent:   true,
					Level:     "WARN",
				}:
				case <-s.ctx.Done():
					return
				}
			}
			return
		}

		line := strings.TrimSuffix(string(lineBytes), "\n")
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}

		// Parse the RFC3339 timestamp injected by K8s (first space-separated token)
		var timestamp time.Time
		content := line
		spaceIdx := strings.Index(line, " ")
		if spaceIdx != -1 {
			tsStr := line[:spaceIdx]
			if parsed, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
				timestamp = parsed
				content = line[spaceIdx+1:]
			}
		}

		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		// Detect severity log level and extract structured fields
		level := detectLogLevel(content)
		fields := extractJSONFields(content)

		select {
		case s.outChan <- LogLine{
			Timestamp: timestamp,
			Pod:       podName,
			Container: containerName,
			Content:   content,
			IsEvent:   false,
			Level:     level,
			Fields:    fields,
		}:
		case <-s.ctx.Done():
			return
		}
	}
}

// extractJSONFields parses scalar fields from a JSON log line for structured filtering.
// Returns nil for non-JSON content.
func extractJSONFields(content string) map[string]string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "{") {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil
	}
	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		switch val := v.(type) {
		case string:
			fields[k] = val
		case float64:
			fields[k] = strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			fields[k] = strconv.FormatBool(val)
		}
	}
	return fields
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// detectLogLevel tries to parse the severity from raw text or structured markers.
// Free-text markers are matched prefix-only to avoid mid-sentence false
// positives (e.g. "no error occurred", "starting debug server"). Structured
// markers (JSON level fields, logfmt level=, bracketed [LEVEL] tags) are matched
// anywhere since they carry negligible false-positive risk.
func detectLogLevel(content string) string {
	lowerContent := strings.ToLower(content)

	// Structured JSON level fields, e.g. {"level":"error"}
	if containsAny(lowerContent, `"level":"error"`, `"level":"fatal"`, `"level":"critical"`) {
		return "ERROR"
	}
	if strings.Contains(lowerContent, `"level":"warn"`) {
		return "WARN"
	}
	if strings.Contains(lowerContent, `"level":"debug"`) {
		return "DEBUG"
	}

	// logfmt level markers, e.g. ts=... level=error msg=...
	if containsAny(lowerContent, "level=error", "level=fatal", "level=critical") {
		return "ERROR"
	}
	if containsAny(lowerContent, "level=warn", "level=warning") {
		return "WARN"
	}
	if strings.Contains(lowerContent, "level=debug") {
		return "DEBUG"
	}

	// Bracketed severity tags emitted by many logging frameworks, e.g. [ERROR]
	if containsAny(lowerContent, "[error]", "[fatal]", "[critical]", "[severe]") {
		return "ERROR"
	}
	if containsAny(lowerContent, "[warn]", "[warning]") {
		return "WARN"
	}
	if strings.Contains(lowerContent, "[debug]") {
		return "DEBUG"
	}

	// Prefix-based severity markers. Includes Go panics, syslog/JUL severities,
	// and Java/Python stack-trace headers.
	if hasAnyPrefix(lowerContent,
		"error ", "error:", "err ", "err:",
		"fatal ", "fatal:", "panic:", "panic ",
		"critical ", "critical:", "severe ", "severe:",
		"exception ", "exception:", "traceback ", "traceback:") {
		return "ERROR"
	}
	if hasAnyPrefix(lowerContent, "warn ", "warn:", "warning ", "warning:") {
		return "WARN"
	}
	if hasAnyPrefix(lowerContent, "debug ", "debug:", "dbg ", "dbg:") {
		return "DEBUG"
	}

	return "INFO"
}
