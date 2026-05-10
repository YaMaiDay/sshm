#!/usr/bin/env sh
set -eu

REPO="YaMaiDay/sshm"
BINARY="sshm"
INSTALL_DIR="${SSHM_INSTALL_DIR:-/usr/local/bin}"
VERSION="${SSHM_VERSION:-latest}"

info() {
  printf '%s\n' "$1"
}

fail() {
  printf '安装失败：%s\n' "$1" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令：$1"
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) fail "暂不支持当前系统：$(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "暂不支持当前架构：$(uname -m)" ;;
  esac
}

need_cmd curl
need_cmd tar

OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)"
fi

[ -n "$VERSION" ] || fail "无法获取最新版本号"

ASSET="sshm_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

info "正在下载 ${BINARY} ${VERSION} (${OS}/${ARCH})..."
curl -fL "$URL" -o "$TMP_DIR/$ASSET" || fail "下载失败：$URL"

tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR" || fail "解压失败"
[ -f "$TMP_DIR/$BINARY" ] || fail "压缩包中没有找到 $BINARY"
chmod +x "$TMP_DIR/$BINARY"

if [ ! -d "$INSTALL_DIR" ]; then
  mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
fi

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

info "安装完成：$INSTALL_DIR/$BINARY"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) info "提示：$INSTALL_DIR 不在 PATH 中，可能需要把它加入 shell 配置。" ;;
esac

info "运行：sshm"
