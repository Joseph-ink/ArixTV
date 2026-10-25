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
sources, duplicates, and all account-based sources. The current CLI keeps the
Phase 0 syntax-only provider-ID behavior because no real runtimes exist yet.
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
