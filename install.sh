#!/usr/bin/env sh
set -eu

REPO="YaMaiDay/sshm"
BINARY="sshm"
INSTALL_DIR="${SSHM_INSTALL_DIR:-}"
VERSION="${SSHM_VERSION:-latest}"

info() {
  printf '%s\n' "$1"
}

fail() {
  printf 'Install failed: %s\n' "$1" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

default_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    printf '%s' "$INSTALL_DIR"
    return
  fi
  case "$OS" in
    darwin)
      if [ -d /opt/homebrew/bin ]; then
        printf '%s' "/opt/homebrew/bin"
      else
        printf '%s' "/usr/local/bin"
      fi
      ;;
    *) printf '%s' "/usr/local/bin" ;;
  esac
}

need_cmd curl
need_cmd tar
need_cmd sed

checksum_cmd() {
  if command -v shasum >/dev/null 2>&1; then
    printf '%s' "shasum -a 256"
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s' "sha256sum"
    return
  fi
  fail "missing SHA256 command: shasum or sha256sum"
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
INSTALL_DIR="$(default_install_dir)"
SHA256_CMD="$(checksum_cmd)"

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" | sed 's#.*/##')"
fi

[ -n "$VERSION" ] || fail "could not resolve latest version"

ASSET="sshm_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

info "Downloading ${BINARY} ${VERSION} (${OS}/${ARCH})..."
curl -fL "$URL" -o "$TMP_DIR/$ASSET" || fail "download failed: $URL"
curl -fL "$CHECKSUMS_URL" -o "$TMP_DIR/checksums.txt" || fail "checksum download failed: $CHECKSUMS_URL"

EXPECTED="$(sed -n "s/[[:space:]][[:space:]]*${ASSET}\$//p" "$TMP_DIR/checksums.txt" | head -n 1)"
[ -n "$EXPECTED" ] || fail "$ASSET was not found in checksums.txt"
ACTUAL="$(cd "$TMP_DIR" && $SHA256_CMD "$ASSET" | sed 's/[[:space:]].*//')"
[ "$EXPECTED" = "$ACTUAL" ] || fail "SHA256 mismatch: expected $EXPECTED, got $ACTUAL"
info "SHA256 verified"

tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR" || fail "extract failed"
[ -f "$TMP_DIR/$BINARY" ] || fail "$BINARY was not found in the archive"
chmod +x "$TMP_DIR/$BINARY"

if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
fi

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

info "Installed: $INSTALL_DIR/$BINARY"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) info "Note: $INSTALL_DIR is not in PATH. You may need to add it to your shell profile." ;;
esac

info "Run: sshm"
