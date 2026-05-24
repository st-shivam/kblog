---
title: Home
layout: home
nav_order: 1
---

# kblog

{: .fs-9 }

A Kubernetes log-tailing TUI that streams pod and deployment logs with real-time K8s event injection, interactive JSON inspection, severity filtering, and themes.
{: .fs-6 .fw-300 }

[Get Started](installation){: .btn .btn-primary .fs-5 .mb-4 .mb-md-0 .mr-2 }
[View on GitHub](https://github.com/st-tripathi/kblog){: .btn .fs-5 .mb-4 .mb-md-0 }

---

Works as both a **standalone CLI tool** and a **drop-in k9s plugin** (press `Shift-L` on any pod or deployment). A single Go binary, no runtime dependencies.

## Why kblog

Most log-viewing workflows for Kubernetes drop you into a raw terminal dump — no interactivity, no context, no way to stay oriented across multiple containers. `kblog` is a purpose-built TUI that keeps you in control.

### Multi-container streaming

Stream all containers in a pod or all replicas of a deployment simultaneously. Every line is prefixed with a stable, color-coded source name so you always know where it came from.

### Live K8s event injection

Pod lifecycle events (`Liveness probe failed`, `OOMKilled`, `Container restarted`, `Scheduled`) are watched in real-time and injected directly into the log timeline as high-visibility alerts — no separate `kubectl get events` tab needed.

### Interactive JSON inspector

Auto-detects structured JSON logs. Press `Enter` on any line to open an overlay modal with syntax-highlighted, scrollable key-value tree. Works on mixed structured/unstructured logs regardless of language or framework.

### Live search with field filtering

Press `/` to open a search bar supporting plain text, regex, and `key=value` field filters (e.g. `level=error request_id=abc`). Non-matching lines fade; matches highlight in real-time.

### Dynamic color themes

Cycle through 5 pre-installed themes instantly with `t`:

| Theme | Description |
|---|---|
| **Midnight** (default) | Deep dark slate with neon purple and cyan accents |
| **Dracula** | Classic Dracula with vibrant pink, purple, and green |
| **Catppuccin Macchiato** | Soft pastel warm blue-grey with mauve highlights |
| **Nord** | Arctic frost cyan and ice-blue |
| **Monokai** | High-contrast neon yellow, green, and pink |

### Clipboard export

Press `c`/`y` on a single line or a multiline visual selection to copy to your system clipboard — works natively on macOS and Linux.

## Performance

| Property | Detail |
|---|---|
| Buffer | 50k-line rolling buffer — memory footprint is bounded regardless of tail duration |
| Filter path | O(1) incremental — new lines are evaluated and appended without re-scanning the buffer |
| Dependencies | Zero runtime dependencies — one statically linked Go binary |

## TUI layout

```
+-------------------------------------------------------------+
| kblog v0.1.0 | Context: production | Pod: auth-service-xyz  |
+------------------------------------------------+------------+
| 12:00:01 [auth] INF -> Initializing DB...      | Containers |
| 12:00:02 [auth] INF -> DB Connection OK        | [x] auth   |
| 12:00:03 [K8S EVENT] Liveness probe succeeded  | [ ] helper |
| 12:00:04 [auth] ERR -> Auth failed for user x  |            |
|   {                                            |            |
|     "user_id": "usr_9013",                     |            |
|     "error_code": "INVALID_TOKEN"              |            |
|   }                                            |            |
| 12:00:05 [auth] INF -> Tailing...              |            |
+------------------------------------------------+------------+
| Filter: /auth-failed                           | Help: q=ex |
+-------------------------------------------------------------+
```
