# Copilot Instructions for dyndns-multiplexer-iid6support

## Project Overview
This Go project is a DynDNS multiplexer/proxy. It receives update requests via HTTP and forwards them to multiple DynDNS providers, supporting both IPv4 and IPv6 (including special handling for provider-specific Interface IDs (IID, "Interface Identifier") in IPv6 addresses). All main logic is implemented in `main.go`. Configuration is handled exclusively via environment variables.

## Example docker-compose Usage
See `examples/docker-compose.yml` for a full setup example. Key points:

- The container exposes port 8080 (mapped to 8085 in the example).
- All configuration is done via environment variables:
  - `USER_NAME`: optional, default 'user'
  - `USER_PASSWORD`: mandatory
  - `USER_DOMAIN_NAME`: optional, default 'any'
  - `PROVIDERS`: JSON array of provider objects (see below)

### Provider Configuration Example
```json
[
  {
    "uri": "https://my.ddns.provider/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "example",
    "passwd": "anotherExample",
    "domain": "exampledomain.my.domain",
    "iid6": "cafe:babe:dead:beef"
  },
  {
    "uri": "https://another.ddns.provider/upd.php?user=<username>&pwd=<passwd>&host=<domain>&ip=<ipaddr>&ip6=<ip6addr>",
    "username": "custom",
    "passwd": "pwd",
    "domain": "anotherdomain.other.domain"
  }
]
```

### Query Parameters (from router/DDNS client)
- `username`: required, must match provider config
- `passwd`: required, must match provider config
- `domain`: required, must match provider config
- `ipaddr`: optional, global IPv4 address (one of `ipaddr` or `ip6addr` required)
- `ip6addr`: optional, global IPv6 address (one of `ipaddr` or `ip6addr` required)
- `ip6lanprefix`: optional, global IPv6 prefix in CIDR notation, e.g. "bab3:d3ad:b33f:ca13::/64"
- `dualstack`: optional

### URI Placeholders (resolved at runtime)
- `<username>`: provider attribute `username`
- `<passwd>`: provider attribute `passwd`
- `<domain>`: provider attribute `domain`
- `<ipaddr>`: query param `ipaddr`
- `<ip6addr>`: if provider attribute `iid6` exists: `ip6lanprefix` + `iid6`; else: query param `ip6addr`
- `<ip6lanprefix>`: query param `ip6lanprefix`
- `<dualstack>`: query param `dualstack`

## Architecture & Components
- **main.go**: Contains all logic (HTTP server, provider handling, query parameter parsing, severity/status tracking, logging).
- **Provider configuration**: Providers are loaded from the `PROVIDERS` environment variable as a JSON array. Each provider has fields like `uri`, `username`, `passwd`, `domain`, and optionally a provider-specific Interface ID (IID, formerly `iid6`).
- **Update endpoint**: The HTTP endpoint `/update` receives requests and forwards them to all configured providers. Query parameters are parsed and validated against provider config.
- **Status/Severity tracking**: DynDNS v2 protocol status codes are mapped to severity levels via a dedicated object. All results are tracked and aggregated for response.

## Key Workflows
- **Startup**:
  1. Set the following environment variables:
  - `USER_NAME`: optional, default 'user' (username for incoming requests)
  - `USER_PASSWORD`: mandatory (password for incoming requests)
  - `USER_DOMAIN_NAME`: optional, default 'any' (domain for incoming requests)
  - `PROVIDERS`: JSON array of provider objects (see example below)
  - `LOG_VERBOSE`: optional, default 'false' (enables verbose logging)
  2. Start with `go run main.go`.
- **Provider URL template**: Provider configuration supports placeholders (`<domain>`, `<ipaddr>`, `<ip6addr>`, `<ip6lanprefix>`, `<dualstack>`, `<username>`, `<passwd>`). At runtime, these are replaced by the corresponding query parameters. For IPv6, if the provider has a provider-specific Interface ID (IID), `<ip6addr>` is constructed from the query param `ip6lanprefix` plus the provider's IID (IID = Interface Identifier).
- **Logging**: Sensitive data in URLs (username, password) is masked in logs.
- **Query parameter validation**: All query parameters are parsed into a struct and validated. Username, password, and domain must match the provider config. Either IPv4 or IPv6 must be present.
- **Error handling**: Errors are logged and returned as HTTP error codes. Missing or invalid config, provider, or query params result in errors.
- **Status/Severity tracking**: All provider responses are mapped to severity levels. Unknown statuses are handled with a fallback severity.

## Development Conventions
- **No external build tools**: Only standard Go tools (`go run`, `go mod tidy`).
- **No Go test files present**: No tests currently defined.
- **Error handling**: All errors are logged and may result in HTTP error codes.
- **Provider handling**: The order of providers is relevant for status determination and aggregation.

## Example Provider Configuration (generic)
```json
[
  {
    "uri": "https://provider.example/update?domain=<domain>&ipaddr=<ipaddr>&ip6addr=<ip6addr>&ip6lanprefix=<ip6lanprefix>&dualstack=<dualstack>&username=<username>&passwd=<passwd>",
    "username": "user",
    "passwd": "pass",
    "domain": "example.com",
    "iid6": "cafe:babe:dead:beef"
  }
]
```

## Guidance for AI Agents
- Changes to provider handling, query param parsing, or logging should always be checked for consistency and security.
- New provider fields must be added to the `Provider` struct and considered during JSON unmarshalling and validation.
- All logic is in a single file, so cross-cutting changes are straightforward but require careful refactoring.
- Severity/status mapping is handled via a dedicated object; ensure new status codes are unique and documented.

## Key Files
- `main.go`: Contains all relevant logic and patterns.
- `go.mod`: Defines the module and Go version.

---
Feedback on unclear or missing sections is welcome!
