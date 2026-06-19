package k8s

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// EventWatcher monitors Kubernetes cluster events for specific resources
type EventWatcher struct {
	clientset  *kubernetes.Clientset
	namespace  string
	targetPods map[string]bool // Set of pod names we are tracking
	podName    string          // If tracking a single pod
	outChan    chan LogLine
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// NewEventWatcher creates a watcher for pod events
func NewEventWatcher(clientset *kubernetes.Clientset, namespace string, podName string, outChan chan LogLine) *EventWatcher {
	targetPods := make(map[string]bool)
	if podName != "" {
		targetPods[podName] = true
	}

	return &EventWatcher{
		clientset:  clientset,
		namespace:  namespace,
		podName:    podName,
		targetPods: targetPods,
		outChan:    outChan,
	}
}

// UpdateTargets dynamically updates the set of pods we are tracking (useful for replica set scaling)
func (w *EventWatcher) UpdateTargets(podNames []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.targetPods = make(map[string]bool)
	for _, name := range podNames {
		w.targetPods[name] = true
	}
}

// Start starts watching events in the background
func (w *EventWatcher) Start(parentCtx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(parentCtx)

	w.wg.Add(1)
	go w.watchLoop()

	return nil
}

// Stop stops the event watcher
func (w *EventWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

func (w *EventWatcher) watchLoop() {
	defer w.wg.Done()

	var opts metav1.ListOptions
	// If watching a single pod, we can let K8s filter on the field selector
	if w.podName != "" {
		opts.FieldSelector = fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", w.podName)
	}

	// Exponential backoff (capped) on watch-establishment failures so a
	// persistently failing watch doesn't spin at a fixed interval forever.
	backoff := initialBackoff

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		watcher, err := w.clientset.CoreV1().Events(w.namespace).Watch(w.ctx, opts)
		if err != nil {
			select {
			case <-w.ctx.Done():
				return
			case <-time.After(backoff):
				backoff = nextBackoff(backoff)
				continue
			}
		}

		// Watch established: reset backoff before consuming the stream.
		backoff = initialBackoff

		w.processEventStream(watcher.ResultChan())
		watcher.Stop()

		select {
		case <-w.ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

func (w *EventWatcher) processEventStream(eventChan <-chan watch.Event) {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			// We only care about Added/Modified events
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}

			k8sEvent, ok := event.Object.(*corev1.Event)
			if !ok {
				continue
			}

			// In-memory filter if we are tracking multiple pods (e.g. deployment)
			if w.podName == "" {
				w.mu.RLock()
				isTracked := w.targetPods[k8sEvent.InvolvedObject.Name]
				w.mu.RUnlock()
				if !isTracked || k8sEvent.InvolvedObject.Kind != "Pod" {
					continue
				}
			}

			// Format event severity level
			level := "INFO"
			if k8sEvent.Type == "Warning" {
				level = "WARN"
			}

			// Determine if it represents a critical error event
			if strings.Contains(k8sEvent.Reason, "Failed") || strings.Contains(k8sEvent.Reason, "BackOff") || strings.Contains(k8sEvent.Reason, "Unhealthy") || strings.Contains(k8sEvent.Reason, "OOM") {
				level = "ERROR"
			}

			// Build nice human-readable message
			msg := fmt.Sprintf("[K8s Event] Reason: %s | Message: %s (Count: %d)", k8sEvent.Reason, k8sEvent.Message, k8sEvent.Count)

			// Push event line into our timeline stream
			select {
			case w.outChan <- LogLine{
				Timestamp: k8sEvent.LastTimestamp.Time,
				Pod:       k8sEvent.InvolvedObject.Name,
				Container: k8sEvent.InvolvedObject.FieldPath, // Often references container name, e.g. "spec.containers{auth}"
				Content:   msg,
				IsEvent:   true,
				Level:     level,
			}:
			case <-w.ctx.Done():
				return
			}
		}
	}
}
