# Installing `khealth`

Pre-built binaries are published on every tagged release at
[github.com/neilfarmer/k8s-health/releases](https://github.com/neilfarmer/k8s-health/releases).

These instructions install to `~/.local/bin/khealth` (no `sudo`). Make sure
that directory is on your `PATH`:

```sh
case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc ;; esac
```

(Replace `~/.bashrc` with `~/.zshrc` if you use zsh. Open a new shell after.)

Pick the section matching your platform.

## Linux (x86_64 / amd64)

```sh
VERSION=v0.1.0
mkdir -p ~/.local/bin
curl -fsSL "https://github.com/neilfarmer/k8s-health/releases/download/${VERSION}/khealth_${VERSION#v}_linux_amd64.tar.gz" \
  | tar -xz -C ~/.local/bin khealth
chmod +x ~/.local/bin/khealth
khealth version
```

## macOS (Apple Silicon / arm64)

```sh
VERSION=v0.1.0
mkdir -p ~/.local/bin
curl -fsSL "https://github.com/neilfarmer/k8s-health/releases/download/${VERSION}/khealth_${VERSION#v}_darwin_arm64.tar.gz" \
  | tar -xz -C ~/.local/bin khealth
chmod +x ~/.local/bin/khealth
khealth version
```

> **Note**: macOS may quarantine the binary. If `khealth version` errors with
> "cannot be opened because the developer cannot be verified", clear the
> quarantine attribute:
>
> ```sh
> xattr -d com.apple.quarantine ~/.local/bin/khealth
> ```

## Verify the download (optional but recommended)

Each release ships a `checksums.txt`. Verify before running:

```sh
VERSION=v0.1.0
curl -fsSL "https://github.com/neilfarmer/k8s-health/releases/download/${VERSION}/checksums.txt" -o /tmp/khealth-checksums.txt
# Linux amd64
cd /tmp && curl -fsSLO "https://github.com/neilfarmer/k8s-health/releases/download/${VERSION}/khealth_${VERSION#v}_linux_amd64.tar.gz"
sha256sum -c --ignore-missing /tmp/khealth-checksums.txt
```

On macOS, swap `sha256sum -c` for `shasum -a 256 -c`.

## Container image

If you'd rather run `khealth` from a container (the in-cluster Job mode uses
this image too):

```sh
docker run --rm -v ~/.kube:/.kube:ro \
  ghcr.io/neilfarmer/k8s-health:v0.1.0 check cluster
```

## Upgrading

Re-run the matching `Linux`/`macOS` block with a newer `VERSION`. The new
binary overwrites `~/.local/bin/khealth` in place.

## Uninstalling

```sh
rm -f ~/.local/bin/khealth
```

## Building from source

If you'd rather compile locally (Go 1.26+):

```sh
git clone https://github.com/neilfarmer/k8s-health.git
cd k8s-health
make build
install -m 0755 dist/khealth ~/.local/bin/khealth
```
