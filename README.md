# dyndns-multiplexer-iid6support

Self-hosted DynDNS multiplexer for routers like Fritzbox. Forwards update requests to multiple providers, supports IPv4/IPv6, custom client addresses via Interface ID (IID) and ISP prefix. Easy config via environment variables and Docker.

## Introduction
Many routers (such as Fritzbox) only allow configuration of a single DynDNS endpoint. Typically, only the router's own IPv6 address is sent to the DynDNS provider—there is no client-side way to forward individual IPv6 addresses from clients (e.g., reverse proxies or internal hosts). Some DynDNS providers offer zone management, and there are various container solutions that periodically check the client's IP address via PULL requests. This project takes a different approach: it does not use PULL, but acts as a self-hosted webhook, allowing you to update multiple DynDNS providers and domains at once. It also enables direct assignment of custom client IPv6 addresses using the Interface ID (IID, "Interface Identifier") and the ISP-assigned IPv6 prefix.

**Successfully tested with ddnss.de as DynDNS provider.**

## Features
- HTTP endpoint `/update` for DynDNS update requests
- Forwards requests to multiple DynDNS providers (configured via environment variable)
- Provider config supports URI templates and placeholders (`<domain>`, `<ipaddr>`, `<ip6addr>`, `<ip6lanprefix>`, `<dualstack>`, `<username>`, `<passwd>`)
- Special IPv6 support: If a provider has an Interface ID (IID), the IPv6 address is constructed from prefix + IID
- Access control via environment variables
- Sensitive data masked in logs
- Robust query parameter validation
- DynDNS v2 protocol status/severity mapping
- Example integration with Fritzbox and similar routers

## Quickstart
1. Set environment variables:
   - `USER_NAME`: optional, default 'user'
   - `USER_PASSWORD`: required
   - `USER_DOMAIN_NAME`: optional, default 'any'
   - `PROVIDERS`: JSON array of provider configs (see below)
2. Start the server:
   ```powershell
   go run main.go
   ```

## Example Provider Configuration
```json
[
  {
    "uri": "https://my.ddns.provider/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "example" ,
    "passwd": "anotherExample",
    "domain": "exampledomain.my.domain",
    "iid6": "cafe:babe:dead:beef"
  },
  {
    "uri": "https://another.ddns.provider/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "custom" ,
    "passwd": "pwd",
    "domain": "anotherdomain.other.domain"
  },
  {
    "uri": "https://www.ddnss.de/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "example" ,
    "passwd": "anotherExample",
    "domain": "worked.with.ddnss.de",
    "iid6": "cafe:babe:dead:beef"
  }
]
```

## Example docker-compose
See `examples/docker-compose.yml` for a full configuration example.

## Environment Variables
- `USER_NAME`: Username for incoming requests (optional)
- `USER_PASSWORD`: Password for incoming requests (required)
- `USER_DOMAIN_NAME`: Domain for incoming requests (optional)
- `PROVIDERS`: JSON array of provider configs
- `LOG_VERBOSE`: Enables verbose logging (optional, default: false)

## How it works
- The `/update` endpoint accepts all relevant query parameters:
  - `username`, `passwd`, `domain` (required)
  - `ipaddr`, `ip6addr` (at least one required)
  - `ip6lanprefix`, `dualstack` (optional)
- Placeholders in the provider URI are replaced at runtime:
  - `<username>`, `<passwd>`, `<domain>`: values from provider config
  - `<ipaddr>`, `<ip6addr>`, `<ip6lanprefix>`, `<dualstack>`: values from query parameters
  - `<ip6addr>`: If `iid6` is set in the provider, `<ip6lanprefix>` + `iid6` is used; otherwise, the value from `ip6addr`

## Development
- All logic is in `main.go`
- Only standard Go tools required (`go run`, `go mod tidy`)

## Requirements for IID (Interface Identifier)
TBD ::: UNPROOVED
To use custom IPv6 addresses for clients, the IID (Interface Identifier, last 64 bits of the IPv6 address) must be stable and predictable. This can be achieved by generating the IID via EUI-64 (based on MAC address) or using stable-privacy mechanisms (recommended). The IID should not change over time, otherwise DynDNS updates will not work reliably.

On routers like FritzBox, the global IPv6 address assigned to a client is typically constructed from the ISP-assigned prefix and the client's IID. For DynDNS to work with custom client addresses, the IID in the router configuration (usually the last 64 bits) must match the IID of the global client IPv6 address. In some cases, you may need to manually set or manipulate the IID in the router to ensure correct mapping and reachability.

## License
MIT

