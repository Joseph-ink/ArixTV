# Initial configuration contract

Phase 0 implements file parsing and validation for the conceptual configuration
in `ARIXTV_SPEC.md` §36. It does not start services, enable providers, check
provider availability, generate secrets, schedule work, or contact any network.
Those capabilities remain in their planned phases.

Run `arixtv config validate --config PATH`. Without the flag, the path is
`./config.toml`, including on macOS. Linux installation/service code will pass
`/etc/arixtv/config.toml` explicitly, so the portable package has no installation
path or systemd dependency. Paths are relative to the caller's working directory.

The parser supports TOML through `github.com/BurntSushi/toml` v1.5.0, the only
external Go module. It is MIT licensed; source and license remain in the Go
module cache, not vendored into this repository. Standard-library code handles
CLI dispatch, network syntax checks, URLs, durations, file input, and tests.

| Section | Defaults | Validation |
|---|---|---|
| `server` | `loopback_address="127.0.0.1"`, `port=8787` | Loopback IP literal; port 1–65535 |
| `lan` | Disabled, empty listeners/networks | Enabled mode requires both lists; listener IPs must be explicit global-unicast literals (includes private addresses), excluding loopback, wildcard, multicast and scoped addresses; networks must be CIDRs |
| `remote` | Disabled, empty origin, `direct_path="/direct"`, trusted proxy `127.0.0.1/32` | Enabled mode requires HTTPS origin and proxies; origin has no user info, path, query or fragment; prefix is a clean absolute non-root path without a trailing slash; proxies are CIDRs |
| `refresh` | `interval="6h"`, global concurrency 8, per-provider concurrency 3 | Positive Go duration and positive concurrency values |
| `providers` | Empty `enabled` list | Unique lowercase IDs beginning with a letter; subsequent letters, digits, `_`, `-` allowed |
| `relay` | Disabled | Boolean setting only; integration is not implemented |

Omitted fields take defaults; empty/comment-only documents are valid. Explicit
zero values do not silently take defaults. Every supplied field is checked even
if its section is disabled. Unknown keys/sections, duplicate TOML definitions,
type mismatches and input above 1 MiB are rejected. Error messages do not echo
configuration values, parser source text, or the supplied file path.

The checked-in `config.example.toml` contains no credentials or enabled providers.
Provider ID validation is syntactic only: `enabled=["xumo"]` does not assert a
provider exists or is ready. Phase 1 will extend the config/domain model and
later phases will check operational compatibility through the provider registry.
CIDR validation checks syntax, not host interfaces, routing, firewall policy or
provider location. No port binding or DNS lookup occurs during validation.

Configuration stores no public bearer tokens. Unknown token/credential fields
are rejected. Local `config.toml` files and runtime directories are ignored by
Git; sanitized examples/fixtures must be explicitly reviewed before inclusion.
