# Copilot Instructions for dyndns-multiplexer-iid6support

## Project Overview
This Go project is a self-hosted DynDNS multiplexer for routers like FritzBox. It receives update requests via HTTP and forwards them to multiple DynDNS providers. It supports IPv4, IPv6, and targeted assignment of client addresses using IPv6 Interface ID (IID) and ISP prefix. Configuration is done exclusively via environment variables and is optimized for Docker.

## Main Features
- HTTP endpoint `/update` for DynDNS updates
- Forwards requests to multiple providers, configured via the `PROVIDERS` environment variable (JSON array)
- Provider URIs support placeholders (`<domain>`, `<ipaddr>`, `<ip6addr>`, `<ip6lanprefix>`, `<dualstack>`, `<username>`, `<passwd>`)
- IPv6: If a provider has an IID (`iid6`), the IPv6 address is constructed from prefix + IID
- Access control via environment variables
- Sensitive data is masked in logs
- Status/severity mapping according to DynDNS v2 protocol
- Example integration with FritzBox and similar routers

## Configuration & Quickstart
- All settings are done via environment variables:
  - `USER_NAME` (optional, default: `user`)
  - `USER_PASSWORD` (required)
  - `USER_DOMAIN_NAME` (optional, default: `dyndns.multiplexer.internal`)
  - `PROVIDERS` (JSON array, see README)
  - `LOG_VERBOSE` (optional, default: false)
- See README for example provider configuration and Docker setup.

## Flow
- `/update` accepts relevant query parameters (see README)
- Placeholders in provider URIs are replaced at runtime
- Provider responses are evaluated:
  - First, check if the `DDNSS-Response` header exists. If so, use it as status. The optional `DDNSS-Message` header is logged.
  - If there is no `DDNSS-Response`, check if a severity attribute exists as a header (e.g. `badauth`). If so, use it.
  - Otherwise, evaluate the response body as before.
- If `LOG_VERBOSE` is enabled, all response headers are logged.

## Development Notes
- All logic is in `main.go`
- Only standard Go tools required (`go run`, `go mod tidy`)
- Always check for consistency and security when changing provider handling, query parsing, or logging
- New provider fields must be added to the `Provider` struct and considered during JSON unmarshalling
- Severity/status mapping is central; document and keep new status codes unambiguous

## Example Provider Configuration
See README.md section "Example Provider Configuration".

## Notes for AI Agents
- Always check for consistency and security when changing provider handling, query parsing, or logging
- New provider fields must be added to the `Provider` struct and considered during JSON unmarshalling
- The order of providers is relevant for status aggregation
- Always write code and documentation in english
