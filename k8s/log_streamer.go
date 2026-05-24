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

// LogStreamer handles concurrent log streaming from K8s pods
type LogStreamer struct {
	clientset *kubernetes.Clientset
	namespace string
	podName   string
	selector  string // Label selector for multi-pod tailing
	tailLines int64
	outChan   chan LogLine
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewLogStreamer creates a new streamer for a specific pod or label selector
func NewLogStreamer(clientset *kubernetes.Clientset, namespace string, podName string, selector string, tailLines int64) *LogStreamer {
	return &LogStreamer{
		clientset: clientset,
		namespace: namespace,
		podName:   podName,
		selector:  selector,
		tailLines: tailLines,
		outChan:   make(chan LogLine, 1000),
	}
}

// Start starts streaming logs concurrently
func (s *LogStreamer) Start(parentCtx context.Context) (<-chan LogLine, error) {
	s.ctx, s.cancel = context.WithCancel(parentCtx)

	var podsToStream []corev1.Pod

	if s.podName != "" {
		// Single pod streaming
		pod, err := s.clientset.CoreV1().Pods(s.namespace).Get(s.ctx, s.podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get pod %s: %w", s.podName, err)
		}
		podsToStream = append(podsToStream, *pod)
	} else if s.selector != "" {
		// Multi-pod streaming via selector
		podList, err := s.clientset.CoreV1().Pods(s.namespace).List(s.ctx, metav1.ListOptions{
			LabelSelector: s.selector,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list pods with selector %s: %w", s.selector, err)
		}
		podsToStream = podList.Items
	} else {
		return nil, fmt.Errorf("either podName or selector must be provided")
	}

	if len(podsToStream) == 0 {
		// Send a dummy event line indicating no pods found
		select {
		case s.outChan <- LogLine{
			Timestamp: time.Now(),
			Content:   "No matching pods found to stream logs.",
			IsEvent:   true,
			Level:     "WARN",
		}:
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		}
		return s.outChan, nil
	}

	// Spawn streams for each container in each pod
	for _, pod := range podsToStream {
		podCopy := pod
		// Find all containers, including init containers
		var containers []string
		for _, c := range podCopy.Spec.InitContainers {
			containers = append(containers, c.Name)
		}
		for _, c := range podCopy.Spec.Containers {
			containers = append(containers, c.Name)
		}

		for _, containerName := range containers {
			s.wg.Add(1)
			go s.streamContainerLogs(podCopy.Name, containerName)
		}
	}

	// Monitor waitgroup to close channel when all streams finish
	go func() {
		s.wg.Wait()
		close(s.outChan)
	}()

	return s.outChan, nil
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

	tail := s.tailLines
	opts := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     true,
		TailLines:  &tail,
		Timestamps: true, // We want K8s to add RFC3339 timestamps for chronological sorting
	}

	// Retry loop for transient connection dropouts
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		req := s.clientset.CoreV1().Pods(s.namespace).GetLogs(podName, opts)
		stream, err := req.Stream(s.ctx)
		if err != nil {
			// Notify about connection failure/retry
			select {
			case s.outChan <- LogLine{
				Timestamp: time.Now(),
				Pod:       podName,
				Container: containerName,
				Content:   fmt.Sprintf("[System] Failed to stream logs: %v. Retrying in 3s...", err),
				IsEvent:   true,
				Level:     "WARN",
			}:
			case <-s.ctx.Done():
				return
			}
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(3 * time.Second):
				continue
			}
		}

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

// detectLogLevel tries to parse the severity from raw text or JSON prefix
func detectLogLevel(content string) string {
	lowerContent := strings.ToLower(content)

	// JSON structured log fields (exact key match, no false positives)
	if strings.Contains(lowerContent, `"level":"error"`) || strings.Contains(lowerContent, `"level":"fatal"`) {
		return "ERROR"
	}
	if strings.Contains(lowerContent, `"level":"warn"`) {
		return "WARN"
	}
	if strings.Contains(lowerContent, `"level":"debug"`) {
		return "DEBUG"
	}

	// Prefix-based severity markers (e.g. "error: ...", "warn something")
	// Avoid mid-sentence substring matches that cause false positives.
	if strings.HasPrefix(lowerContent, "error ") || strings.HasPrefix(lowerContent, "error:") ||
		strings.HasPrefix(lowerContent, "err ") || strings.HasPrefix(lowerContent, "err:") ||
		strings.HasPrefix(lowerContent, "fatal ") || strings.HasPrefix(lowerContent, "fatal:") {
		return "ERROR"
	}
	if strings.HasPrefix(lowerContent, "warn ") || strings.HasPrefix(lowerContent, "warn:") ||
		strings.HasPrefix(lowerContent, "warning ") || strings.HasPrefix(lowerContent, "warning:") {
		return "WARN"
	}
	if strings.HasPrefix(lowerContent, "debug ") || strings.HasPrefix(lowerContent, "debug:") ||
		strings.HasPrefix(lowerContent, "dbg ") || strings.HasPrefix(lowerContent, "dbg:") ||
		strings.Contains(lowerContent, " debug ") || strings.Contains(lowerContent, " dbg ") {
		return "DEBUG"
	}

	return "INFO"
}
