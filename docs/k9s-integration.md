
# k9s Integration

- [How k9s plugins work](#how-k9s-plugins-work)
- [Automatic setup](#automatic-setup)
- [Manual setup](#manual-setup)
- [Plugin variables](#plugin-variables)
- [Troubleshooting](#troubleshooting)

---

## How k9s plugins work

When you highlight a resource in k9s and press a configured hotkey:

1. k9s suspends its render loop.
2. It executes the plugin command, handing over full terminal control.
3. k9s passes the resource name, namespace, and cluster context as flags.
4. When the command exits (press `q` in `kblog`), k9s restores the previous view.

---

## Automatic setup

The install script and `make install` handle the plugin automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/st-shivam/kblog/main/install.sh | bash
# or
make install
```

Restart k9s after installation. The `Shift-L` binding will appear in the hotkey bar on Pod and Deployment views.

---

## Manual setup

Add the following to `~/.config/k9s/plugins.yaml` (create it if it doesn't exist):

```yaml
plugins:
  kblog-pod:
    shortCut: Shift-L
    confirm: false
    description: "kblog"
    scopes:
      - pods
    command: kblog
    background: false
    args:
      - --context
      - $CONTEXT
      - --namespace
      - $NAMESPACE
      - --pod
      - $NAME

  kblog-deployment:
    shortCut: Shift-L
    confirm: false
    description: "kblog"
    scopes:
      - deployments
    command: kblog
    background: false
    args:
      - --context
      - $CONTEXT
      - --namespace
      - $NAMESPACE
      - --deployment
      - $NAME
```

Restart k9s to load the configuration.

---

## Plugin variables

k9s injects these automatically when invoking the plugin:

| Variable | Description |
|---|---|
| `$CONTEXT` | Active kubeconfig cluster context |
| `$NAMESPACE` | Active namespace |
| `$NAME` | Name of the highlighted resource |

---

## Troubleshooting

**`Shift-L` does nothing or shows "command not found"**

k9s cannot find the `kblog` binary in its PATH. Install it to a directory that's on your PATH, or use the absolute path in `plugins.yaml`:

```yaml
command: /usr/local/bin/kblog
```

**Logs or events fail to load**

Your kubeconfig context may lack permissions. Verify:

```bash
kubectl auth can-i get pods --namespace <ns>
kubectl auth can-i get events --namespace <ns>
```

**Colors look wrong or the modal is distorted**

Ensure your terminal supports 24-bit truecolor. Set `COLORTERM=truecolor` in your shell profile. Tested terminals: iTerm2, Alacritty, WezTerm, GNOME Terminal, Kitty.

---

## Plugin config path by OS

| OS | Path |
|---|---|
| macOS / Linux | `~/.config/k9s/plugins.yaml` |
| macOS (Homebrew k9s) | `~/Library/Application Support/k9s/plugins.yaml` |
