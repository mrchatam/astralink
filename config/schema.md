# AstraLink configuration schema

## Global

| Key | Description |
|-----|-------------|
| `CONFIG_VERSION` | Schema version for migrations |
| `TRANSPORT` | Fixed: `multipath_quic_dns` |
| `MODE` | `simple` or `advanced` |

## Client transport

| Key | Simple default | Advanced |
|-----|----------------|----------|
| `MAX_ACTIVE_PATHS` | 1 | 4+ |
| `MAX_STANDBY_PATHS` | 1 | 2+ |
| `FEC_ENABLED` | false | true optional |
| `PACKET_DUPLICATION_COUNT` | 1 | 2+ |
| `CONGESTION_PROFILE` | conservative | adaptive |
| `MAX_BUNDLE_BYTES` | 512 | 1200 |
| `AUTHORITY_ENDPOINTS` | empty | multi-node list |

## Server authority

| Key | Description |
|-----|-------------|
| `AUTHORITY_MODE` | `single`, `multi`, `anycast` |
| `AUTHORITY_PEERS` | Peer authority addresses |
| `ANYCAST_ENABLED` | Future anycast NS flag |
