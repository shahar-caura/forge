#!/usr/bin/env bash
set -euo pipefail

REPO="shahar-caura/forge"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)
        echo "Error: unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

case "$OS" in
    darwin|linux) ;;
    *)
        echo "Error: unsupported OS: $OS" >&2
        exit 1
        ;;
esac

echo "Detected: ${OS}/${ARCH}"

# Fetch latest release tag.
echo "Fetching latest release..."
TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release tag" >&2
    exit 1
fi

VERSION="${TAG#v}"
echo "Latest version: ${VERSION}"

# Download and extract.
TARBALL="forge_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${TARBALL}"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMPDIR}/${TARBALL}"

echo "Extracting..."
tar xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

# Install.
echo "Installing to ${INSTALL_DIR}/forge..."
if [ -w "$INSTALL_DIR" ]; then
    mv "${TMPDIR}/forge" "${INSTALL_DIR}/forge"
else
    sudo mv "${TMPDIR}/forge" "${INSTALL_DIR}/forge"
fi
chmod +x "${INSTALL_DIR}/forge"

# Verify.
if "${INSTALL_DIR}/forge" version >/dev/null 2>&1; then
    echo ""
    "${INSTALL_DIR}/forge" version
    echo ""
else
    echo "Warning: forge installed but 'forge version' failed" >&2
fi

# Check runtime dependencies.
echo "Checking runtime dependencies..."
MISSING=""
for cmd in git gh claude; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        MISSING="${MISSING}  - ${cmd}\n"
    fi
done

if [ -n "$MISSING" ]; then
    echo ""
    echo "Warning: missing runtime dependencies:"
    printf "$MISSING"
    echo ""
    echo "Install them before running forge:"
    echo "  git:    https://git-scm.com"
    echo "  gh:     https://cli.github.com"
    echo "  claude: https://claude.ai/download"
fi

echo ""
echo "Next steps:"
echo "  cd your-repo && forge init"
