# Phase 2 completion report

Completed 2026-10-26 (Asia/Hong_Kong). Phase 1 was approved. This change completes
HTTP serving/network policy only. Phase 3 is unstarted; no real providers, media
proxy, relay integration, refresh scheduler or installer were added. No commit,
staging or push was performed. The repository was clean when work began.

## Additions and changed files

```text
cmd/arixtv/main.go                  # signal-aware entry point
internal/
├── cli/
│   ├── cli.go                     # serve dispatch and context propagation
│   ├── cli_test.go                # staged-command expectation updated
│   └── serve_test.go              # new CLI validation/cancellation tests
├── config/
│   ├── config.go                  # HTTP defaults and LAN advertised origin
│   ├── validate.go                # Phase 2 validation integration
│   ├── config_test.go             # valid LAN fixture now has explicit origin
│   ├── http.go                    # new bounded HTTP/config contract
│   └── http_test.go               # new validation tests
├── server/
│   ├── app.go                     # initialization, state and injected interfaces
│   ├── catalog.go                 # immutable catalog and snapshot/status reload
│   ├── connections.go             # shared TCP admission bound
│   ├── handler.go                 # routes, auth, playlist, resolve, health
│   ├── limiter.go                 # bounded per-IP token buckets
│   ├── path.go                    # strict route/opaque-ID parsing
│   ├── policy.go                  # listener ACL and trusted proxy policy
│   ├── serve.go                   # binding, lifecycle and graceful shutdown
│   ├── server_test.go
│   ├── limits_reload_test.go
│   └── lifecycle_test.go
└── servicenotify/
    ├── notify.go                  # portable notifier/watchdog boundary
    ├── notify_linux.go            # optional Unix-datagram implementation
    ├── notify_other.go            # Darwin/other-platform no-op
    ├── notify_test.go
    ├── notify_linux_test.go
    └── notify_other_test.go
```

Also updated: `config.example.toml`, `PLANS.md`, `README.md`, `ARIXTV_SPEC.md`,
`docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`, `docs/NGINX.md`, and
`docs/SECURITY.md`. This report is new. Phase 1 auth/channel/store/atomicfile/
statefile/filelock/logsafe/provider implementations, go.mod/go.sum, and
`docs/FASTCHANNELS_MAPPING.md` are unchanged.

## Server and listener architecture

`arixtv serve [--config PATH]` loads strict config, operationally validates enabled
provider runtimes, checks token/snapshot/refresh state, binds every configured
socket, then begins HTTP serving/readiness. Any bind failure closes previous
sockets. Startup errors are static and secret-safe. Serving does not create token
or snapshot state. The production registry is deliberately empty in Phase 2;
`providers.enabled=[]` is valid and serves empty local health. Unknown/unavailable
or account-based enabled providers fail startup. Tests inject mock runtimes and
provision sanitized snapshots/tokens through Phase 1 APIs.

A literal loopback bind (default 127.0.0.1:8787) carries Remote Direct, health and
the optional internal resolver. Explicit LAN binds on the same port carry Direct
only. No wildcard is the default or accepted as a loopback binding. The embedding
Serve API also validates binding kinds/addresses. Each listener receives its own
policy handler, while tokens/catalog/admission limits are shared.

There is no provider discovery call or internal provider-refresh scheduler.

## LAN ACL and trusted proxy/client-IP design

LAN requests require actual socket peer IP membership in configured CIDRs and a
valid public token. Forwarding headers never affect LAN policy or limiter identity.
IPv4, IPv6 and IPv4-mapped addresses are handled consistently; explicitly mapped
IPv4 prefixes are normalized where unambiguous. The source port is not an identity.

Loopback handlers require an actual loopback socket peer. Only configured trusted
proxy ranges may supply a single X-Forwarded-For IP literal with exactly one
`X-Forwarded-Proto: https`. Duplicate/comma-separated/malformed trusted values are
404. Untrusted forwarding values are ignored. Forwarded, Real-IP and forwarded-host
headers are never alternate identities/origins. Requests without proxy headers
retain socket identity; local token-authenticated debugging remains possible when
Remote Direct is enabled. Tokens are always required.

Health/internal additionally reject forwarding metadata, helping prevent accidental
exposure through a broadly configured proxy. Nginx must still proxy only Direct:
a local proxy stripping all client context is indistinguishable from a local user.
The documented config preserves the path and uses `$remote_addr`, not an arbitrary
forwarding chain.

## Advertised origins, playlist generation and strict paths

Remote output uses `remote.public_origin`; LAN output uses new explicit
`lan.public_origin`. Both use `remote.direct_path` (default /direct). Host,
forwarded-host, TLS claims, local interfaces and source IPs never generate URLs.
One advertised LAN origin is used across all allowed LAN clients/interfaces.

LAN-enabled old configs must add the advertised HTTP origin. Disabled LAN defaults
remain compatible. HTTP settings have conservative bounded defaults; see the full
field/default/bounds table in CONFIGURATION.md. Origins have no credentials,
query/path/fragment or invalid port. Prefixes use unreserved ASCII components.

M3U is sorted by provider/channel ID and includes only validated clear_direct
channels. Ordinary HLS standard encryption may remain direct; relay-only/DRM/
unsupported/unknown entries never appear. It contains resolver URLs, no EPG/XMLTV
fields, no signed upstream playback URLs and no internal credential. Responses
use audio/x-mpegurl, no-store, nosniff and no-referrer headers. Rendering writes
one text entry at a time with a counting pass for Content-Length/HEAD, avoiding a
full playlist buffer per client. No playlist file is generated in this phase.

There is no ServeMux normalization/redirect. Exact GET/HEAD paths reject percent
escapes (including encoded slash/backslash/dot aliases), empty/dot components,
backslashes, controls, non-ASCII raw path components, query strings and absolute-form
request targets. Ordinary path-safe IDs stay readable; others use canonical
`~`+base64url components. Decoded opaque IDs are lookup keys, never filesystem paths.
Malformed HTTP rejected by net/http itself may receive standard 400/431 before the
handler; ambiguous application paths receive 404 without Location canonicalization.

## Route table and authentication

| Route, GET/HEAD only | Listener | Policy | Result |
|---|---|---|---|
| `<direct>/s/<TOKEN>/playlist.m3u` | LAN | Allowed socket CIDR + public token | 200 deterministic Direct M3U |
| Same playlist | Loopback | Remote enabled, loopback peer + public token; strict trusted metadata if supplied | 200 using configured HTTPS origin |
| `<direct>/s/<TOKEN>/play/<provider>/<channel>` | Both Direct policies | Same token/network checks, known clear_direct channel/runtime | Bodyless 302 with fresh validated Location |
| `/internal/<INTERNAL_SECRET>/play/<provider>/<channel>` | Loopback only, no forwarding metadata | Separate injected internal validator; clear_direct/clear_relay_only only | Bodyless 302; CLI defaults disabled |
| `/healthz` | Loopback only, no forwarding metadata | No public token; local access only | 200 minimal structured ready/degraded status |
| `/direct/healthz`, admin/refresh/config/unknown routes | None | Never exposed | 404 |

The Phase 1 token store verifies each public request. Creation/rotation/revocation
is immediately observed; no positive authentication cache keeps revoked tokens
alive. Invalid, malformed, missing, unknown or revoked tokens use the same fixed
404 response without token-name/format/state details. Corrupt token state denies
access and remains preserved. Methods other than GET/HEAD and requests with bodies
are 404; bodies are not read and their connections are closed.

Resolve rechecks listener policy, token, strict provider/channel lookup, channel
capability, runtime availability, concurrency, context deadline and the returned
Stream's URL/type/DRM/capability/expiry. There is no resolved-URL cache. Temporary
failures, invalid results and provider panics become sanitized 503 with Retry-After
30; no raw upstream error or response is returned. A fresh URL appears only in
Location, never an HTML response body, log record or persisted snapshot.

## Internal relay and health foundations

Internal auth is an injected `VerifyInternal` interface, with a SHA-256 constant-
time digest validator available. It requires a separate `irv_`+256-bit base64url
namespace; public `atv_` tokens cannot substitute. Nil validation denies all internal
requests. The CLI does not provision internal credentials; this is deferred to
Phase 7, as allowed by the task. No relay.m3u or relay engine exists. The loopback
listener/peer restriction is independent of credential validation.

Health contains service state, version, total normalized/direct channel counts,
enabled/provider snapshot counts, per-provider safe IDs/states/generations,
healthy/stale/failed/unknown/refreshing counts and successful-refresh timestamps.
It omits URLs, paths, tokens, cookies and raw errors. Persisted Phase 1 refresh
observations are consumed; old healthy data becomes stale after two configured
refresh intervals. This is local/catalog health, not a claim that CDN playback
works. Public/LAN health routes do not exist.

## Rate limiting, concurrency and reload

Defaults are 128 accepted TCP connections shared across listeners, 8 active
resolver calls, per-IP playlist refill 30/minute, resolver refill 60/minute and
burst 5. Extra sockets close before handler goroutines are created. Resolver
admission is nonblocking with no extra worker/waiting queue; full capacity returns
503. Resolve context timeout defaults to 10 seconds.

One bounded limiter map (default 1024 identities) holds pre-auth/playlist/resolve
buckets. Pre-auth rate/burst are four times the larger class rate and burst;
admission/capacity denial here stays 404 independent of token validity. After valid
authentication, per-class exhaustion is 429 with Retry-After 30. TTL eviction occurs
at capacity; active entries are not displaced to restore credits. Default TTL is
10 minutes and must cover full burst refill. Actual/validated client IPs identify
buckets, never ports or untrusted forwarded values.

The main service loop polls snapshots/refresh observations every 5 seconds by
default. Inode/mtime/size stamps avoid reparsing unchanged files. Enabled provider
files only are candidates; temporary files are ignored. Strict Phase 1 readers
validate changed data, increasing generations replace immutable catalog views,
and aggregate cached channel count is capped at 100000. Corrupt startup data fails
startup; invalid/deleted/regressing runtime data preserves in-memory LKG and degrades
local health. Stat/read races retry on a later poll. No persisted file is reset.

## Graceful shutdown and systemd

SIGINT/SIGTERM cancellation denies new work, cancels base/resolver contexts, closes
listeners and applies one shared Shutdown deadline (default 10 seconds). On expiry,
sockets close and a safe error returns; the CLI exits rather than waiting forever.
Future providers must honor context: Go cannot forcibly kill arbitrary provider
code. Serve worker goroutines, reload timers and heartbeat timers are owned and
joined/stopped. The service has no independent always-running scheduler.

Linux optionally sends READY, WATCHDOG and STOPPING over NOTIFY_SOCKET (pathname
or abstract Unix datagram). WATCHDOG_PID is respected; half WATCHDOG_USEC drives
heartbeats in the main service loop. Writes are bounded to 200ms; Darwin is a no-op.
This supervises main-loop responsiveness, not provider playback. There is no
libsystemd dependency and no final systemd unit file in Phase 2. Protocol references
are linked from ARCHITECTURE.md.

## Dependencies and licensing

No dependency was added. The Go standard library provides HTTP, routing, IP parsing,
limiting, cancellation, Unix datagrams and signal handling. BurntSushi/toml v1.5.0
(MIT) remains the sole production third-party dependency; existing notices are
unchanged. Pinned development-only govulncheck v1.9.0 remains BSD-3-Clause. ArixTV's
own license remains an owner decision before distribution. No upstream provider
code, proprietary secret or account credential was used/copied.

## Exact validation and results

Compiler: `go version go1.27.3 darwin/arm64`. All required local checks passed:

```sh
gofmt -w .
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go test -race ./...
go mod tidy
go mod verify
go run ./cmd/arixtv config validate --config config.example.toml
GOTOOLCHAIN=local bin/tools/govulncheck -test ./...
```

All 12 internal packages passed tests/race checks. The scanner reported **No
vulnerabilities found**. Normal tests use local recorders, temp directories, mock
runtimes and local TCP/Unix sockets; they do not access the live internet. The
entry point has native process smoke coverage in addition to CLI unit tests.

Cross-builds/test compilation, all successful with CGO disabled:

```sh
for target in linux/amd64 linux/arm64 darwin/arm64; do
  GOTOOLCHAIN=local GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build ./...
  GOTOOLCHAIN=local GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go build -trimpath -o "bin/arixtv-${target%/*}-${target#*/}" ./cmd/arixtv
  GOTOOLCHAIN=local GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go test -c -o "bin/notify-test-${target%/*}-${target#*/}" ./internal/servicenotify
done
go version -m bin/arixtv-darwin-arm64
```

Compiler metadata confirmed Go 1.27.3. Linux binaries and Linux notifier tests were
compiled, not executed on this macOS host. Existing CI is configured to run native
Ubuntu/macOS tests and the separate live scanner; no hosted run occurred without a
push. Native Darwin process smoke tests launched `serve --config <temporary file>`,
checked local health, public/internal auth denial and secret-safe logs, then verified
clean exit under **both SIGINT and SIGTERM**. Serving created no data/state files.

Tests cover valid/invalid/revoked tokens; strict path/method/body denial; IPv4/IPv6
ACLs and spoofing; one-hop forwarded parsing/identity; configured origins and Host
injection; deterministic classification-filtered M3U; fresh empty-body 302s; safe
provider errors/panics; cancellation/deadline/concurrency; internal/health isolation;
rate refill/eviction/cap; connection limits; bind rollback; managed reload; corrupt
state/LKG; metadata health/staleness; graceful shutdown and notification lifecycles.

Repository hygiene checks include tracked/new-file diff whitespace, credential/
private-key/JWT/URL-userinfo patterns, ignored runtime/build/config paths, no obsolete
compiler baseline, unchanged foundations/mapping/dependency graph, and no tracked
runtime/generated data. Generated build/test binaries stay in ignored bin/. All
state fixtures are test-local and sanitized.

## Owner review before Phase 3

Review the deliberate Phase 2 choices: one required LAN advertised origin, strict
canonical opaque-channel encoding, separate default-disabled internal validator,
explicit pre-auth versus authenticated rate responses, and LKG/degraded reload
behavior. No new product-policy ambiguity blocks Phase 3. ArixTV license selection
is still required before distribution; it was not authorized here.

The next planned phase is **Phase 3 — First providers**, after owner approval and
fresh protocol/permission verification. Selection from the inventory and independent
provider implementation have not begun. Full relay credential setup remains Phase 7.
