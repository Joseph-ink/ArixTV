# Initial configuration contract

The configuration package implements file parsing and validation for the conceptual configuration
in `ARIXTV_SPEC.md` §36. It does not start services, enable providers, check
provider availability, generate secrets, schedule work, or contact any network.
Operational commands remain in their planned phases.

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
| `lan` | Disabled, empty origin/listeners/networks | Enabled mode requires an explicit HTTP `public_origin` and both lists; listener IPs must be explicit global-unicast literals (includes private addresses), excluding loopback, wildcard, multicast and scoped addresses; networks must be CIDRs |
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
provider exists or is ready. Phase 1 adds storage paths and an explicit operational registry check;
later commands will invoke that check when real runtimes exist.
CIDR validation checks syntax, not host interfaces, routing, firewall policy or
provider location. No port binding or DNS lookup occurs during validation.

Configuration stores no public bearer tokens. Unknown token/credential fields
are rejected. Local `config.toml` files and runtime directories are ignored by
Git; sanitized examples/fixtures must be explicitly reviewed before inclusion.

## Phase 1 storage and registry integration

The configuration schema now additionally accepts:

```toml
[storage]
data_dir = "./data"
state_dir = "./state"
```

These are defaults for omitted fields, so existing Phase 0 configurations still
validate. Empty paths, NUL/newlines, type errors and unknown fields are rejected.
Paths are relative to the invoking process's working directory; no path is assumed
to be `/etc/arixtv/` inside the core. Linux deployment will pass absolute data/state
paths later. Config validation creates no files, checks no interfaces and performs
no DNS/provider requests. Examples contain no enabled providers or tokens.

Later operational commands can call `Config.ValidateProviders(registry)` in
addition to structural validation. It rejects missing runtimes, disabled/unsupported
sources, duplicates, and all account-based sources. `config validate` keeps syntax-only provider-ID behavior. `serve` additionally
invokes operational registry validation. Phases 3A/3B register experimental
anonymous/free `vizio` and `xumo` runtimes; no other real runtime is available.
Provider IDs are shared with the domain/store validation: lowercase letter first,
then letters, digits, `_` or `-`, at most 64 bytes.

The state layout supplied to these foundations is:

```text
<data_dir>/providers/<id>.json       # versioned LKG channel snapshot
<data_dir>/providers/.<id>.lock      # persistent provider mutation lock inode
<state_dir>/tokens.json             # names, SHA-256 hashes, lifecycle timestamps
<state_dir>/tokens.lock             # persistent token mutation lock inode
<state_dir>/refresh.lock            # global scheduled/manual refresh lock inode
<state_dir>/refresh/<id>.json        # attempt/failure observation, no channel data
```

No tokens, relay secrets, provider sessions, account settings or later-phase
service configuration were added to TOML. Snapshot and token JSON versions are 1;
unknown versions require explicit migration/recovery, never silent reset.


## Phase 2 serving configuration

`arixtv serve [--config PATH]` uses the same config selection as validation.
The existing loopback address/port are retained; enabled LAN addresses use the
same port and must be explicit unicast IPs. At most 16 LAN addresses, 64 enabled
providers and 256 CIDRs in each LAN/trusted-proxy list are accepted. Every listener must bind successfully before readiness;
a bind failure closes previously opened listeners. No wildcard default is used.

LAN now requires **an explicit advertised origin when enabled**:

```toml
[lan]
enabled = true
listen_addresses = ["192.168.50.10"]
allow_networks = ["192.168.50.0/24"]
public_origin = "http://192.168.50.10:8787"
```

This is the deliberate Phase 2 migration for older LAN-enabled configs: add
`lan.public_origin` before validation/serving. Disabled LAN configs retain their
old defaults. One advertised LAN origin is used for all authorized LAN clients,
even with multiple listener addresses/CIDRs. It may be an operator-configured DNS
name; the server does not derive it from Host, forwarded headers, interfaces or
source addresses. LAN uses HTTP; Remote Direct uses the configured HTTPS origin.
Origins have no credentials, path, query, fragment or invalid port, and are limited
to 2048 bytes. Select an
origin actually reachable by the intended clients; validation performs no DNS or
reachability probe. `remote.direct_path` is the shared Direct prefix for both
modes, even with remote mode disabled. It must be a clean, non-root absolute path
of unreserved ASCII components, at most 128 bytes, outside `/internal` and `/healthz`.

The optional `[http]` table defaults to:

| Field | Default | Accepted bounds/meaning |
|---|---|---|
| `read_header_timeout` | `5s` | Duration 1ms–24h |
| `read_timeout` | `15s` | Duration 1ms–24h; at least header timeout |
| `write_timeout` | `30s` | Duration 1ms–24h; greater than resolver timeout |
| `idle_timeout` | `60s` | Duration 1ms–24h |
| `resolve_timeout` | `10s` | Context deadline, 1ms–24h |
| `shutdown_timeout` | `10s` | Common graceful shutdown deadline, 1ms–24h |
| `reload_interval` | `5s` | Snapshot/observation polling interval, 1ms–24h |
| `max_header_bytes` | `16384` | 1024–1048576 bytes; standard net/http framing allowance also applies |
| `max_connections` | `128` | 1–4096 accepted TCP connections across all listeners |
| `resolver_concurrency` | `8` | 1–64 active Resolve calls; no waiting work queue |
| `playlist_per_minute` | `30` | Per effective IP token-bucket refill rate, 1–60000 |
| `resolve_per_minute` | `60` | Per effective IP token-bucket refill rate, 1–60000 |
| `rate_burst` | `5` | Per-class burst, 1–1000 |
| `rate_max_clients` | `1024` | 1–65536 limiter identities, bounded storage |
| `rate_client_ttl` | `10m` | 1ms–24h; must cover a full burst refill period at the slower rate |

All supplied fields remain strictly validated, including disabled sections.
Unknown keys, zero/unbounded values and type mismatches are rejected. The example
contains every HTTP default. No plaintext public/internal secret config was added.
`relay.enabled` does not provision credentials or enable a relay engine in Phase 2.

Only enabled, available anonymous provider runtimes are operationally accepted.
The production CLI accepts `providers.enabled=["vizio", "xumo"]`, either provider
alone, or the default empty list. Neither needs provider-specific TOML settings.
Vizio uses the anonymous US mobile catalog; Xumo uses the public US market 10006
catalog with no fixed geo ID, country override or account configuration. Xumo's
Direct classification currently covers only its documented reference identity;
other unverified channels remain unknown. Enabling a provider only permits
serving existing snapshots; it does not discover/refresh in the daemon. The refresh
CLI remains Phase 4. Test/embedding code can pass a mock registry to exercise
persisted snapshots. Missing enabled snapshots permit degraded/empty serving;
corrupt/unsupported snapshot, token or observation state fails startup without
rewriting files. The catalog is capped at 100000 aggregate channels. Configuration/provider-registry changes require restart; token creation, rotation
and revocation are observed immediately by token verification. Completed snapshots and
refresh observations reload live at the configured interval.
