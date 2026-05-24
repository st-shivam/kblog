
# kblog

A Kubernetes log-tailing TUI — stream pod and deployment logs with live K8s event injection, JSON inspection, severity filtering, and themes. One binary, no runtime dependencies.

[Install now](installation.md) · [View on GitHub](https://github.com/st-shivam/kblog)

---

```
+-------------------------------------------------------------+
| kblog v0.1.0 | Context: prod | Deployment: auth-service     |
+------------------------------------------------+------------+
| 12:00:01 [auth-7f9] INF  DB connection pool OK | Containers |
| 12:00:02 [auth-7f9] INF  Listening on :8080    | [x] auth   |
| 12:00:03 [K8S EVENT] ⚠ Liveness probe failed  | [x] sidecar|
| 12:00:04 [auth-7f9] ERR  Token validation fail |            |
|   {                                            |            |
|     "user_id": "usr_9013",                     |            |
|     "error": "token_expired"                   |            |
|   }                                            |            |
| 12:00:05 [auth-a2c] INF  Starting replica...   |            |
+------------------------------------------------+------------+
| Search: level=error                   f=follow | q to quit  |
+-------------------------------------------------------------+
```

---

## What makes it different

Most Kubernetes log tools dump raw text into a terminal. `kblog` is a full interactive TUI — you stay oriented across containers, clusters, and chaos.

**Concurrent multi-container streaming** — stream every container in a pod or every replica of a deployment in one unified view. Each source gets a stable color-coded prefix so you always know where a line came from.

**Live K8s event injection** — `OOMKilled`, `CrashLoopBackOff`, liveness probe failures, scheduling events injected inline as they happen. No more switching to a separate `kubectl get events` tab while something is on fire.

**Interactive JSON inspector** — press `Enter` on any structured log line to open a syntax-highlighted key-value overlay. Works on mixed streams regardless of language or logging framework.

**Live search and field filtering** — press `/` to filter by plain text, regex, or `key=value` pairs (e.g. `level=error service=auth`). Non-matching lines dim instantly.

**Five color themes** — Midnight (default), Dracula, Catppuccin Macchiato, Nord, Monokai. Cycle with `t`.

**Clipboard copy** — `c` on a single line. `v` for a visual selection, then `c` to copy the block.

---

## Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/st-shivam/kblog/main/install.sh | bash
```

Then tail a pod:

```bash
kblog --pod <name> --namespace <namespace>
```

Or tail a full deployment across all replicas:

```bash
kblog --deployment <name> --namespace <namespace>
```

> The install script places the binary in `/usr/local/bin` and sets up the k9s plugin automatically.
> See [Installation](installation.md) for custom paths and version pinning.

---

## k9s integration

Press `Shift-L` on any Pod or Deployment in k9s to open `kblog` in place of the built-in log view. Press `q` to return to k9s. The install script wires this up automatically.
