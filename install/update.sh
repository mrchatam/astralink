#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/opt/astralink}"
CONFIG="${INSTALL_DIR}/server_config.toml"

log() { echo "[astralink-update] $*"; }

if [[ ! -f "$CONFIG" ]]; then
  echo "Missing $CONFIG — run install-server.sh first" >&2
  exit 1
fi

backup() {
  local f="$1"
  [[ -f "$f" ]] && cp -a "$f" "${f}.bak.$(date +%s)"
}

backup "$CONFIG"
log "Config backed up. Replace binaries in ${INSTALL_DIR} and restart astralink.service"
systemctl restart astralink.service 2>/dev/null || log "systemd unit not found; restart manually"
