---
title: Architecture
nav_order: 5
---

# Architecture
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Overview

`kblog` is structured into three layers connected by Go channels:

```
+--------------------------------------------------------+
|                 TUI Rendering Layer                    |
|       (Bubbletea Elm loop + Lipgloss styling)          |
+--------------------------------------------------------+
                           ▲
                           │  LogLine channel stream
                           ▼
+--------------------------------------------------------+
|              Concurrency Coordination                  |
|       (WaitGroup + context-aware channel writes)       |
+--------------------------------------------------------+
                           ▲
                           │  Background subscriptions
                           ▼
+--------------------------------------------------------+
|                Kubernetes I/O Layer                    |
|       (client-go log streaming + API event watch)      |
+--------------------------------------------------------+
```

---

## Package structure

```
main.go                  CLI flags, K8s client init, goroutine wiring, Bubbletea launch
k8s/
  client.go              Kubeconfig loading, namespace resolution
  log_streamer.go        Concurrent pod/container log streaming; LogLine type
  event_watcher.go       K8s event watch → LogLine injection into shared channel
tui/
  bubble.go              Bubbletea Model, Update (key handling), View, clipboard
  viewport.go            Log buffer, filter/search, render
  sidebar.go             Container toggle list
  json_inspector.go      JSON pretty-print modal overlay
  styles.go              Lipgloss themes, global style vars, InitStyles()
plugin/plugins.yaml      k9s plugin definition (Shift-L → kblog)
```

---

## Channel pipeline

```
LogStreamer → logChan → pipe goroutine → sharedLogChan ← EventWatcher
                                                ↓
                                          TUI waitForLogs()
```

`waitForLogs` returns a `tea.Cmd` that reads one `LogLine` from `sharedLogChan`, delivering it into the Bubbletea Elm loop. This keeps K8s I/O off the render goroutine.

---

## Coordinated shutdown

When the user exits, the root context (`bgCtx`) is cancelled. A `sync.WaitGroup` ensures `sharedLogChan` is closed only after both the log pipe goroutine and the event watcher have fully exited:

```go
var mergeWg sync.WaitGroup
mergeWg.Add(2)

go func() {          // log pipe
    defer mergeWg.Done()
    for line := range logChan {
        select {
        case sharedLogChan <- line:
        case <-bgCtx.Done():
            return
        }
    }
}()

go func() {          // event watcher stop tracker
    defer mergeWg.Done()
    <-bgCtx.Done()
    watcher.Stop()
}()

go func() {          // final closer
    mergeWg.Wait()
    close(sharedLogChan)
}()
```

All channel writes across the codebase use a `select`/`ctx.Done()` guard so that a stalled TUI cannot cause shutdown to deadlock.

---

## Two-phase filter pipeline

The log buffer maintains two slices:

- **`AllLines`** — full immutable buffer, capped at 50k lines.
- **`FilteredLines`** — active view after applying level, search, and container filters.

**Fast path — `AddLineWithFilter`** (called on every incoming log line): evaluates the new line against current filters in O(1) and appends directly to `FilteredLines` if it matches. No buffer scan.

**Slow path — `UpdateFilters`** (called only when the user changes a filter): re-scans all 50k lines in `AllLines` and rebuilds `FilteredLines`. Triggered by search query changes, level filter key presses, or container sidebar toggles.

---

## ANSI-aware modal overlay

The JSON inspector modal is rendered over log lines that contain ANSI color escape codes. Standard byte-index slicing would break mid-escape-sequence and corrupt the terminal output.

`kblog` solves this with a character-cell engine:

```go
type ansiCell struct {
    char  rune
    style string  // accumulated ANSI prefix for this cell
}
```

1. **`parseAnsiString`** — converts a styled string into a slice of `ansiCell`. Each cell carries the full ANSI prefix active at that character position. Style state propagates forward until a reset sequence (`\x1b[0m`) is encountered.
2. **Overlay** — splicing is performed on the cell slice, ensuring escape sequences are never split.
3. **`cellsToString`** — compiles the cell slice back into a terminal string with a trailing `\x1b[0m` reset.

---

## Theme system

All color and style constants live in `tui/styles.go` as package-level `lipgloss.Style` variables. `InitStyles(theme)` recompiles every variable for the selected theme. Render code references these globals directly — no inline color literals in render paths.

This makes theme cycling instantaneous: one call to `InitStyles` + a full re-render, no state to thread through the model.
