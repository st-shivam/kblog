---
title: Usage
nav_order: 3
---

# Usage
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## CLI flags

```
kblog --pod <name> [flags]
kblog --deployment <name> [flags]
```

| Flag | Description | Example |
|---|---|---|
| `--pod` | Pod name to tail | `--pod auth-service-6f9d-x7b2p` |
| `--deployment` | Deployment name (tails all replicas) | `--deployment auth-service` |
| `--namespace` | Kubernetes namespace | `--namespace production` |
| `--context` | Kubeconfig context | `--context prod-cluster` |
| `--theme` | Initial color theme | `--theme dracula` |

`--pod` and `--deployment` are mutually exclusive. One must be provided.

---

## Examples

Tail a single pod:

```bash
kblog --pod my-pod-name --namespace my-namespace
```

Tail all replicas of a deployment with a specific context:

```bash
kblog --deployment auth-service --namespace production --context prod-cluster
```

Launch with the Nord theme:

```bash
kblog --pod payment-gateway --theme nord
```

Available themes: `midnight` (default), `dracula`, `catppuccin`, `nord`, `monokai`.

---

## Keyboard shortcuts

### Navigation

| Key | Action |
|---|---|
| `up` / `k` | Move cursor up / scroll up |
| `down` / `j` | Move cursor down / scroll down |
| `pgup` / `pgdown` | Page up / page down |
| `f` / `F` | Follow — scroll to latest line and lock auto-scroll |
| `Tab` | Switch focus between log viewport and sidebar |

### Filtering & search

| Key | Action |
|---|---|
| `/` | Open search bar (plain text, regex, or `key=value`) |
| `0` | Level filter: **All** |
| `1` | Level filter: **ERROR** only |
| `2` | Level filter: **WARN** and above |
| `3` | Level filter: **DEBUG** and above |
| `l` / `L` | Toggle container filter sidebar |
| `Space` | Toggle container stream on/off (in sidebar focus) |

### Viewing & copying

| Key | Action |
|---|---|
| `Enter` | Open / close JSON inspector modal on selected line |
| `v` | Start visual multiline selection |
| `c` / `y` | Copy selected line(s) to clipboard |
| `w` / `W` | Toggle line wrap |
| `s` / `S` | Toggle sort order (oldest ↑ / newest ↓) |
| `t` / `T` | Cycle through themes |

### General

| Key | Action |
|---|---|
| `q` | Quit (return to shell or k9s) |
| `Esc` | Cancel visual selection, or quit |

---

## Search syntax

The search bar (activated with `/`) supports three modes:

**Plain text** — case-insensitive substring match:
```
auth failed
```

**Regex** — full Go regex syntax:
```
(ERROR|WARN).*timeout
```

**Field filter** — space-separated `key=value` pairs matching structured log fields:
```
level=error service=auth request_id=abc123
```

Field filters work on any key in a JSON-formatted log line. Multiple pairs are ANDed together.

---

## JSON inspector

When `kblog` detects that a log line contains a JSON payload, pressing `Enter` opens an overlay modal showing the parsed key-value tree with syntax highlighting. Press `Enter` again or `Esc` to close.

The inspector works on mixed logs — lines without JSON pass through normally.

---

## Container sidebar

In deployments with multiple containers (or multi-replica deployments), the sidebar (toggle with `l`) lists each streaming source. Press `Space` to enable or disable individual containers from the log feed.

All containers are enabled by default.
