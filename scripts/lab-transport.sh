#!/usr/bin/env bash
# Deterministic + real-network transport lab (3-resolver A/B FEC on/off).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo "== Phase 4A: deterministic transport suite =="
go test ./internal/transport/... -v -count=1 \
  -run 'RTT|Stickiness|Multipath|FEC|SentVsAcked|Promotion|GroupAck|Terminal|Timeout|Inflight|Control|DNSDelivered|E2E'
go test ./internal/client/... -v -count=1 -skip ExchangeUDP \
  -run 'Transport|StreamAck|Timeout|SingleTransport'
go test ./internal/udpserver/... -v -count=1 -run 'Transport|FEC|Parity'
go test ./internal/vpnproto/... -count=1 -run FEC
go test ./internal/authority/... -count=1

echo ""
echo "== Phase 4B: real-network A/B (manual) =="
echo "See docs/LAB_PASS_CRITERIA.md for pass thresholds."
if [[ -f config/advanced.client.toml ]]; then
  echo "FEC ON:  TRANSPORT_FEC / FEC_ENABLED=true in advanced.client.toml"
  echo "FEC OFF: FEC_ENABLED=false"
  echo "Run: ./bin/astralink-client -config config/advanced.client.toml"
  echo "Collect: astralink_transport and astralink_path log lines (5 min each)."
echo "Record results in docs/LIVE_VALIDATION_REPORT.md"
else
  echo "Skip: config/advanced.client.toml not present"
fi

echo ""
echo "== Phase 4C: validation report template =="
echo "See docs/LIVE_VALIDATION_REPORT.md (fill FEC on/off tables after live runs)."

echo "== Lab complete =="
