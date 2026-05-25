#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/astralink}"
CONFIG="${INSTALL_DIR}/server_config.toml"

log() { echo "[astralink-update] $*"; }

if [[ ! -f "$CONFIG" ]]; then
  echo "Missing $CONFIG — run install-server.sh first" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run as root (sudo)." >&2
  exit 1
fi

backup() {
  local f="$1"
  [[ -f "$f" ]] && cp -a "$f" "${f}.bak.$(date +%s)"
}

backup "$CONFIG"
log "Config backed up. Replace binaries in ${INSTALL_DIR} and restart astralink.service"
if ! command -v systemctl >/dev/null 2>&1; then
  log "systemctl not found; restart manually"
  exit 0
fi

if systemctl list-unit-files --all 2>/dev/null | grep -q '^astralink\.service'; then
  systemctl restart astralink.service
else
  log "astralink.service not found; restart manually"
fi
