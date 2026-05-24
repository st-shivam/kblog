
# Installation

- [Requirements](#requirements)
- [Option 1 — Install script (recommended)](#option-1--install-script-recommended)
- [Option 2 — Release tarball](#option-2--release-tarball)
- [Option 3 — Build from source](#option-3--build-from-source)

---

## Requirements

- **OS:** Linux x86-64 or ARM64
- **Cluster access:** A valid kubeconfig (`~/.kube/config` or `$KUBECONFIG`)
- **Go 1.26+** only needed if building from source

---

## Option 1 — Install script (recommended)

Downloads the right binary for your platform and installs the k9s plugin automatically.

```bash
curl -fsSL https://raw.githubusercontent.com/st-shivam/kblog/main/install.sh | bash
```

**Custom install directory** (no `sudo`):

```bash
INSTALL_DIR=$HOME/.local/bin \
  curl -fsSL https://raw.githubusercontent.com/st-shivam/kblog/main/install.sh | bash
```

If `~/.local/bin` is not on your PATH yet:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
# zsh users: use ~/.zshrc instead
```

**Pin to a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/st-shivam/kblog/main/install.sh | VERSION=v0.1.0 bash
```

---

## Option 2 — Release tarball

Download the archive for your platform from the [Releases page](https://github.com/st-shivam/kblog/releases):

| Platform | Archive |
|---|---|
| Linux x86-64 | `kblog_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `kblog_<version>_linux_arm64.tar.gz` |

Extract and install:

```bash
tar -xzf kblog_<version>_linux_amd64.tar.gz
sudo mv kblog /usr/local/bin/
```

Install the k9s plugin bundled in the archive:

```bash
cat plugin/plugins.yaml >> ~/.config/k9s/plugins.yaml
```

---

## Option 3 — Build from source

Requires [Go 1.26+](https://go.dev/doc/install).

```bash
git clone https://github.com/st-shivam/kblog.git
cd kblog

# No sudo — install to a local directory
make install INSTALL_PATH=$HOME/.local/bin

# Or system-wide
sudo make install
```

`make install` builds the binary, copies it to the target directory, and installs the k9s plugin in one step.

---

## Verify

```bash
kblog --help
```

You should see the full usage output. If the command isn't found, ensure the install directory is on your `PATH`.
