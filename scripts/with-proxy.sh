#!/usr/bin/env bash
# Use local SOCKS proxy for Go module downloads and network commands.
export ALL_PROXY="${ALL_PROXY:-socks5h://127.0.0.1:10808}"
export HTTP_PROXY="${HTTP_PROXY:-socks5h://127.0.0.1:10808}"
export HTTPS_PROXY="${HTTPS_PROXY:-socks5h://127.0.0.1:10808}"
exec "$@"
