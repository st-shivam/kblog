package k8s

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

func makePod(name string, initContainers, containers []string) corev1.Pod {
	pod := corev1.Pod{}
	pod.Name = name
	for _, c := range initContainers {
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{Name: c})
	}
	for _, c := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: c})
	}
	return pod
}

func TestNextBackoff_DoublesAndCaps(t *testing.T) {
	cases := []struct {
		in, want time.Duration
	}{
		{1 * time.Second, 2 * time.Second},
		{2 * time.Second, 4 * time.Second},
		{16 * time.Second, 30 * time.Second}, // 32s capped to 30s
		{30 * time.Second, 30 * time.Second}, // stays at cap
	}
	for _, c := range cases {
		if got := nextBackoff(c.in); got != c.want {
			t.Errorf("nextBackoff(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestEnumerateContainers_InitAndRegularAcrossPods(t *testing.T) {
	pods := []corev1.Pod{
		makePod("pod-a", []string{"init-a"}, []string{"app", "sidecar"}),
		makePod("pod-b", nil, []string{"app"}),
	}
	got := enumerateContainers(pods)

	want := []string{"pod-a/init-a", "pod-a/app", "pod-a/sidecar", "pod-b/app"}
	if len(got) != len(want) {
		t.Fatalf("enumerateContainers returned %d entries, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].key() != w {
			t.Errorf("entry %d = %q, want %q", i, got[i].key(), w)
		}
	}
}

func TestPodNamesOf(t *testing.T) {
	pods := []corev1.Pod{
		makePod("pod-a", nil, []string{"app"}),
		makePod("pod-b", nil, []string{"app"}),
	}
	got := podNamesOf(pods)
	if len(got) != 2 || got[0] != "pod-a" || got[1] != "pod-b" {
		t.Errorf("podNamesOf = %v, want [pod-a pod-b]", got)
	}
}

// claimNewStreams must not re-claim a pod/container that is already being
// streamed (so the poller doesn't double-stream existing replicas) while
// claiming the containers of newly-appeared pods — the core of #4.
func TestClaimNewStreams_Dedup(t *testing.T) {
	s := &LogStreamer{streaming: map[string]bool{}}

	// Pretend pod-a/app is already streaming; the poller must not re-claim it.
	s.streaming["pod-a/app"] = true

	pods := []corev1.Pod{
		makePod("pod-a", nil, []string{"app"}),      // already streaming
		makePod("pod-b", nil, []string{"app", "x"}), // new replica
	}
	fresh := s.claimNewStreams(enumerateContainers(pods))

	wantFresh := []string{"pod-b/app", "pod-b/x"}
	if len(fresh) != len(wantFresh) {
		t.Fatalf("claimNewStreams returned %d entries, want %d: %v", len(fresh), len(wantFresh), fresh)
	}
	for i, w := range wantFresh {
		if fresh[i].key() != w {
			t.Errorf("fresh[%d] = %q, want %q", i, fresh[i].key(), w)
		}
	}

	// All three containers should now be marked streaming.
	for _, key := range []string{"pod-a/app", "pod-b/app", "pod-b/x"} {
		if !s.streaming[key] {
			t.Errorf("expected %q to be marked streaming", key)
		}
	}

	// A second poll with the same pods must claim nothing new.
	if again := s.claimNewStreams(enumerateContainers(pods)); len(again) != 0 {
		t.Errorf("second claim should be empty, got %v", again)
	}
}

// SetOnPodsChanged should be invoked with the full current pod set so the event
// watcher's targets stay in sync after rollouts/scaling (#4).
func TestSpawnStreamsForPods_ReportsCurrentPods(t *testing.T) {
	s := &LogStreamer{streaming: map[string]bool{}}

	// Pre-seed all containers so spawnStreamsForPods launches no real goroutines
	// (a nil clientset stream would panic); we only assert the callback here.
	s.streaming["pod-a/app"] = true
	s.streaming["pod-b/app"] = true

	var reported [][]string
	s.SetOnPodsChanged(func(pods []string) {
		reported = append(reported, append([]string(nil), pods...))
	})

	pods := []corev1.Pod{
		makePod("pod-a", nil, []string{"app"}),
		makePod("pod-b", nil, []string{"app"}),
	}
	s.spawnStreamsForPods(pods)

	if len(reported) != 1 {
		t.Fatalf("onPodsChanged called %d times, want 1", len(reported))
	}
	if len(reported[0]) != 2 || reported[0][0] != "pod-a" || reported[0][1] != "pod-b" {
		t.Errorf("onPodsChanged got %v, want [pod-a pod-b]", reported[0])
	}
}

func TestExtractJSONFields_BasicTypes(t *testing.T) {
	fields := extractJSONFields(`{"level":"error","request_id":"abc-123","retries":3,"ok":true}`)
	if fields == nil {
		t.Fatal("expected non-nil fields for valid JSON")
	}
	tests := []struct{ key, want string }{
		{"level", "error"},
		{"request_id", "abc-123"},
		{"retries", "3"},
		{"ok", "true"},
	}
	for _, tt := range tests {
		if got := fields[tt.key]; got != tt.want {
			t.Errorf("fields[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestExtractJSONFields_NonJSON(t *testing.T) {
	cases := []string{
		"plain text log line",
		"2024-01-01T00:00:00Z INFO starting server",
		"",
		"[ERROR] something failed",
	}
	for _, c := range cases {
		if got := extractJSONFields(c); got != nil {
			t.Errorf("extractJSONFields(%q) = %v, want nil", c, got)
		}
	}
}

func TestExtractJSONFields_NestedObjectsSkipped(t *testing.T) {
	// Nested objects are not scalar — should not appear in fields
	fields := extractJSONFields(`{"msg":"hi","nested":{"a":1}}`)
	if fields == nil {
		t.Fatal("expected non-nil fields")
	}
	if _, ok := fields["nested"]; ok {
		t.Error("nested object should be skipped")
	}
	if fields["msg"] != "hi" {
		t.Errorf("fields[msg] = %q, want %q", fields["msg"], "hi")
	}
}

func TestExtractJSONFields_InvalidJSON(t *testing.T) {
	if got := extractJSONFields(`{"level":"error"`); got != nil {
		t.Errorf("invalid JSON should return nil, got %v", got)
	}
}

func TestExtractJSONFields_LeadingWhitespace(t *testing.T) {
	fields := extractJSONFields(`  {"level":"warn","svc":"auth"}`)
	if fields == nil {
		t.Fatal("expected non-nil for JSON with leading whitespace")
	}
	if fields["level"] != "warn" {
		t.Errorf("fields[level] = %q, want %q", fields["level"], "warn")
	}
}

func TestDetectLogLevel(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		// JSON structured fields
		{`{"level":"error","msg":"boom"}`, "ERROR"},
		{`{"level":"fatal","msg":"dead"}`, "ERROR"},
		{`{"level":"warn","msg":"low disk"}`, "WARN"},
		{`{"level":"debug","msg":"trace"}`, "DEBUG"},
		// Prefix-based detection
		{"error connecting to db", "ERROR"},
		{"error: connection refused", "ERROR"},
		{"err something went wrong", "ERROR"},
		{"err: timeout", "ERROR"},
		{"fatal: out of memory", "ERROR"},
		{"warn: disk almost full", "WARN"},
		{"warn threshold exceeded", "WARN"},
		{"warning: config missing", "WARN"},
		{"debug: entering handler", "DEBUG"},
		{"debug mode enabled", "DEBUG"},
		{"dbg: skip", "DEBUG"},
		// #14: panic / additional severity prefixes
		{"panic: runtime error: invalid memory address", "ERROR"},
		{"critical: disk failure", "ERROR"},
		{"severe: subsystem down", "ERROR"},
		{"exception in thread \"main\"", "ERROR"},
		{"traceback (most recent call last):", "ERROR"},
		// #14: bracketed [LEVEL] tags
		{"[ERROR] something failed", "ERROR"},
		{"[FATAL] giving up", "ERROR"},
		{"2024-01-01 [WARN] disk almost full", "WARN"},
		{"[DEBUG] entering loop", "DEBUG"},
		// #14: logfmt level= markers
		{"ts=2024-01-01 level=error msg=boom", "ERROR"},
		{"ts=2024-01-01 level=warn msg=low", "WARN"},
		{"ts=2024-01-01 level=debug msg=trace", "DEBUG"},
		// #14: JSON critical
		{`{"level":"critical","msg":"meltdown"}`, "ERROR"},
		// Default INFO
		{"INFO server started", "INFO"},
		{"just a plain log line", "INFO"},
		// #7 + false-positive regression cases: mid-sentence severity words must NOT trigger
		{" debug mode enabled", "INFO"},   // #7: mid-sentence debug is NOT DEBUG
		{"starting debug server", "INFO"}, // #7
		{"enabled dbg endpoint", "INFO"},  // #7
		{"connection error rate: 0%", "INFO"},
		{"no error occurred", "INFO"},
		{"0 warnings issued", "INFO"},
		{"processed request without warnings", "INFO"},
		{"caught no exception during run", "INFO"}, // mid-sentence exception must NOT trigger
	}
	for _, tt := range cases {
		if got := detectLogLevel(tt.input); got != tt.want {
			t.Errorf("detectLogLevel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
