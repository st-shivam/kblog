package k8s

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// syntheticEvent builds a watch.Event wrapping a corev1.Event for use in tests.
func syntheticEvent(eventType watch.EventType, podName, reason, message string, k8sType string) watch.Event {
	return watch.Event{
		Type: eventType,
		Object: &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{Name: reason},
			InvolvedObject: corev1.ObjectReference{
				Kind: "Pod",
				Name: podName,
			},
			Reason:        reason,
			Message:       message,
			Type:          k8sType,
			LastTimestamp: metav1.Time{Time: time.Now()},
			Count:         1,
		},
	}
}

func newTestWatcher(podName string) (*EventWatcher, chan LogLine) {
	out := make(chan LogLine, 10)
	ctx, cancel := context.WithCancel(context.Background())
	w := &EventWatcher{
		podName:    podName,
		targetPods: map[string]bool{podName: true},
		outChan:    out,
		ctx:        ctx,
		cancel:     cancel,
	}
	return w, out
}

func TestProcessEventStream_AddsNormalEvent(t *testing.T) {
	w, out := newTestWatcher("my-pod")
	ch := make(chan watch.Event, 1)
	ch <- syntheticEvent(watch.Added, "my-pod", "Scheduled", "pod scheduled", "Normal")
	close(ch)

	w.processEventStream(ch)

	select {
	case line := <-out:
		if line.Pod != "my-pod" {
			t.Errorf("Pod = %q, want %q", line.Pod, "my-pod")
		}
		if line.Level != "INFO" {
			t.Errorf("Level = %q, want INFO", line.Level)
		}
		if !line.IsEvent {
			t.Error("expected IsEvent = true")
		}
	default:
		t.Error("expected a LogLine to be emitted")
	}
}

func TestProcessEventStream_WarningEventGetsWARNLevel(t *testing.T) {
	w, out := newTestWatcher("my-pod")
	ch := make(chan watch.Event, 1)
	ch <- syntheticEvent(watch.Added, "my-pod", "SomeWarning", "disk pressure", "Warning")
	close(ch)

	w.processEventStream(ch)

	select {
	case line := <-out:
		if line.Level != "WARN" {
			t.Errorf("Level = %q, want WARN", line.Level)
		}
	default:
		t.Error("expected a LogLine")
	}
}

func TestProcessEventStream_FailedReasonGetsERRORLevel(t *testing.T) {
	w, out := newTestWatcher("my-pod")
	ch := make(chan watch.Event, 1)
	ch <- syntheticEvent(watch.Added, "my-pod", "BackOff", "back-off restarting", "Warning")
	close(ch)

	w.processEventStream(ch)

	select {
	case line := <-out:
		if line.Level != "ERROR" {
			t.Errorf("Level = %q, want ERROR", line.Level)
		}
	default:
		t.Error("expected a LogLine")
	}
}

func TestProcessEventStream_IgnoresDeletedEvents(t *testing.T) {
	w, out := newTestWatcher("my-pod")
	ch := make(chan watch.Event, 1)
	ch <- syntheticEvent(watch.Deleted, "my-pod", "Scheduled", "pod deleted", "Normal")
	close(ch)

	w.processEventStream(ch)

	select {
	case line := <-out:
		t.Errorf("unexpected LogLine emitted for Deleted event: %+v", line)
	default:
		// correct: Deleted events are ignored
	}
}

func TestProcessEventStream_FiltersUnknownPod(t *testing.T) {
	out := make(chan LogLine, 10)
	ctx, cancel := context.WithCancel(context.Background())
	w := &EventWatcher{
		podName:    "", // deployment mode: filter via targetPods map
		targetPods: map[string]bool{"tracked-pod": true},
		outChan:    out,
		ctx:        ctx,
		cancel:     cancel,
	}

	ch := make(chan watch.Event, 2)
	ch <- syntheticEvent(watch.Added, "other-pod", "Scheduled", "some event", "Normal")
	ch <- syntheticEvent(watch.Added, "tracked-pod", "Scheduled", "tracked event", "Normal")
	close(ch)

	w.processEventStream(ch)

	if len(out) != 1 {
		t.Fatalf("expected 1 LogLine, got %d", len(out))
	}
	line := <-out
	if line.Pod != "tracked-pod" {
		t.Errorf("Pod = %q, want %q", line.Pod, "tracked-pod")
	}
}

func TestProcessEventStream_ContextCancelStopsLoop(t *testing.T) {
	w, out := newTestWatcher("my-pod")
	// Cancel the context before processing starts
	w.cancel()

	ch := make(chan watch.Event) // unbuffered, never sends

	done := make(chan struct{})
	go func() {
		w.processEventStream(ch)
		close(done)
	}()

	select {
	case <-done:
		// correct: loop exited on ctx.Done()
	case <-time.After(time.Second):
		t.Error("processEventStream did not exit on context cancellation")
	}

	if len(out) != 0 {
		t.Errorf("expected no LogLines, got %d", len(out))
	}
}

func TestUpdateTargets_ReplacesExistingSet(t *testing.T) {
	w, _ := newTestWatcher("pod-a")
	w.UpdateTargets([]string{"pod-b", "pod-c"})

	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.targetPods["pod-a"] {
		t.Error("pod-a should have been removed")
	}
	if !w.targetPods["pod-b"] || !w.targetPods["pod-c"] {
		t.Error("pod-b and pod-c should be tracked")
	}
}
