# AstraLink

AstraLink is a DNS transport system built around **Multipath QUIC over DNS**.

## Core capabilities

- Parallel resolver channels with adaptive scheduling
- Redundancy and congestion control
- Optional FEC for high-loss paths
- EDNS-aware control bundling
- Single-authority and multi-authority scaling in one protocol

## Quick start (one VPS / one domain)

```bash
cd astralink
go build -o astralink-server ./cmd/server
go build -o astralink-client ./cmd/client
sudo bash install/install-server.sh --local
```

Configure DNS delegation for your tunnel subdomain, then connect with `config/simple.client.toml`.

## Documentation

- [Naming](docs/NAMING.md)
- [Transport v1](docs/TRANSPORT_V1.md)
- [Quick setup](docs/QUICKSTART_ONE_VPS.md)

## License

MIT (inherits upstream components; see LICENSE).
