package k8s

import (
	"testing"
)

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
		// Contains-based debug detection
		{" debug mode enabled", "DEBUG"},
		// Default INFO
		{"INFO server started", "INFO"},
		{"just a plain log line", "INFO"},
		// False-positive regression cases: mid-sentence error/warn words must NOT trigger
		{"connection error rate: 0%", "INFO"},
		{"no error occurred", "INFO"},
		{"0 warnings issued", "INFO"},
		{"processed request without warnings", "INFO"},
	}
	for _, tt := range cases {
		if got := detectLogLevel(tt.input); got != tt.want {
			t.Errorf("detectLogLevel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
