package k8s

import (
	"os"
	"path/filepath"
	"testing"
)

const testKubeconfig = `apiVersion: v1
kind: Config
clusters:
- name: c1
  cluster:
    server: https://127.0.0.1:6443
    insecure-skip-tls-verify: true
contexts:
- name: ctx-a
  context:
    cluster: c1
    user: u1
    namespace: team-a
- name: ctx-b
  context:
    cluster: c1
    user: u1
    namespace: team-b
current-context: ctx-a
users:
- name: u1
  user:
    token: fake-token
`

func writeKubeconfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	return path
}

func TestLoadClient_NamespaceFromCurrentContext(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t, testKubeconfig))

	info, err := LoadClient("", "")
	if err != nil {
		t.Fatalf("LoadClient: %v", err)
	}
	if info.Namespace != "team-a" {
		t.Errorf("namespace = %q, want team-a (from current context)", info.Namespace)
	}
	if info.Clientset == nil {
		t.Error("expected non-nil Clientset")
	}
}

func TestLoadClient_ContextOverride(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t, testKubeconfig))

	info, err := LoadClient("ctx-b", "")
	if err != nil {
		t.Fatalf("LoadClient: %v", err)
	}
	if info.Namespace != "team-b" {
		t.Errorf("namespace = %q, want team-b (from ctx-b)", info.Namespace)
	}
}

func TestLoadClient_ExplicitNamespaceWins(t *testing.T) {
	t.Setenv("KUBECONFIG", writeKubeconfig(t, testKubeconfig))

	info, err := LoadClient("", "explicit-ns")
	if err != nil {
		t.Fatalf("LoadClient: %v", err)
	}
	if info.Namespace != "explicit-ns" {
		t.Errorf("namespace = %q, want explicit-ns", info.Namespace)
	}
}

func TestLoadClient_MissingKubeconfigErrors(t *testing.T) {
	// Point KUBECONFIG at a path that does not exist.
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "does-not-exist"))

	if _, err := LoadClient("", ""); err == nil {
		t.Error("expected an error when kubeconfig is missing")
	}
}
