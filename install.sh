#!/usr/bin/env bash
# install.sh — instala o binário devon em ~/.local/bin
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/ElioNeto/devon/main/install.sh | sh
#
set -euo pipefail

REPO="ElioNeto/devon"
INSTALL_DIR="${HOME}/.local/bin"
BINARY="devon"

# Determina version (último tag via GitHub API, ou fallback)
VERSION="${VERSION:-latest}"

echo "=> Instalando ${BINARY} (${VERSION}) ..."

# Detecta OS/ARCH
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="x86_64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "ERRO: arquitetura não suportada: $ARCH"; exit 1 ;;
esac
case "$OS" in
  linux)  ;;
  darwin) ;;
  *)      echo "ERRO: sistema operacional não suportado: $OS"; exit 1 ;;
esac

TARBALL="devon_$(echo "$OS" | tr '[:lower:]' '[:upper:]')_${ARCH}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${TARBALL}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"
fi

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

echo "=> Baixando ${DOWNLOAD_URL} ..."
curl -fsSL -o "${TMPDIR}/${TARBALL}" "$DOWNLOAD_URL"

echo "=> Extraindo para ${INSTALL_DIR}/${BINARY} ..."
mkdir -p "$INSTALL_DIR"
tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"
mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"

echo "=> ${BINARY} instalado: $(${INSTALL_DIR}/${BINARY} --version 2>&1 || echo ok)"
echo "=> Certifique-se de que ${INSTALL_DIR} está no seu PATH."
