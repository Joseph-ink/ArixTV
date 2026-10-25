# Phase 0.1 and Phase 1 completion report

Completed 2026-10-26 (Asia/Hong_Kong). Phase 0 was approved by the owner. Work
stops after Phase 1; Phase 2/3 have not started. No commit or push was made.

## Phase 0.1 security/toolchain migration

Minimum supported Go, CI and the future release-build baseline are **1.27.3**.
The pre-existing owner edit to `go.mod` already set that version and was preserved.
Obsolete compiler compatibility claims/checks were removed from README, PLANS and
the Phase 0 report. CI explicitly selects Go 1.27.3 and sets `GOTOOLCHAIN=local`.
Release publishing remains Phase 8; no release workflow was prematurely added.
The baseline is relevant to provider HTTPS through Go's crypto/x509 mitigation
for CVE-2026-111943 ([official release history](https://go.dev/doc/devel/release#go1.27.3)).

Baseline formatting, tests, vet, module verification and all three cross-builds
passed before Phase 1 work began. Official `govulncheck` **v1.9.0** was installed
and scanned successfully, then rerun against the completed Phase 1 code.

CI now tests natively on Ubuntu and macOS 14, including race checks; a separate
Ubuntu job runs the official scanner. Tool code is pinned to
`golang.org/x/vuln/cmd/govulncheck@v1.9.0` with its versioned dependency graph and
normal Go checksum verification. There is no `@latest` installation and no new
production dependency. The vulnerability database intentionally stays live:
scanner configuration is reproducible, database findings can change over time.
The scanner is development-only, BSD-3-Clause, and is not distributed with ArixTV.

## Relevant repository tree and new code

```text
internal/
├── atomicfile/
│   ├── directory.go
│   ├── write.go
│   └── write_test.go
├── auth/
│   ├── store.go
│   ├── token.go
│   └── token_test.go
├── channel/
│   ├── channel.go
│   ├── channel_test.go
│   ├── stream.go
│   └── types.go
├── cli/                         # existing Phase 0 CLI, unchanged
├── config/
│   ├── config.go                # extended
│   ├── config_test.go           # existing
│   ├── storage_test.go          # new integration tests
│   └── validate.go              # extended
├── filelock/
│   ├── flock_other.go
│   ├── flock_unix.go
│   ├── lock.go
│   └── lock_test.go
├── logsafe/
│   ├── log.go
│   └── log_test.go
├── provider/
│   ├── registry.go
│   └── registry_test.go
├── statefile/
│   ├── json.go
│   └── json_test.go
└── store/
    ├── metadata.go
    ├── refreshlock.go
    ├── snapshot.go
    ├── snapshot_test.go
    └── write.go
```

Every listed file in the new packages, plus `config/storage_test.go` and this
report, was created. Existing files changed: `.github/workflows/ci.yml`,
`AGENTS.md`, `ARIXTV_SPEC.md`, `PLANS.md`, `README.md`, `THIRD_PARTY_NOTICES.md`,
`config.example.toml`, `docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`,
`docs/PHASE0_REPORT.md`, `docs/PROVIDERS.md`, `docs/SECURITY.md`, `internal/config/config.go` and
`internal/config/validate.go`. `go.mod` retains the owner-supplied baseline edit;
`go.sum` and the production dependency graph are unchanged.
`docs/FASTCHANNELS_MAPPING.md` is unchanged as requested. Current approved owner
policy in the specification governs future implementation over historical mapping
questions/conditional candidates.

## Architecture and domain model

`channel.Channel` deliberately persists only provider/channel identity, display
metadata, stream/protection/capability classification, optional SHA-256 metadata
fingerprint and last-seen timestamp. It has explicit JSON field names and rejects
missing identity, control characters/invalid UTF-8, excessive metadata, unsafe
artwork query/userinfo, invalid fingerprints and contradictory classifications.
No arbitrary payload, header, account/session blob or playback URL field exists.

Capabilities are `clear_direct`, `clear_relay_only`, `drm`, `unsupported`, `unknown`.
Protection is separate: `clear`, `hls_standard_encrypted`, `widevine`, `playready`,
`fairplay`, `other_drm`, `unknown`. Ordinary AES-128 HLS can be clear-direct when
normal clients can play it; same-egress/header/identity limitations must be
relay-only. Direct eligibility rejects unknown and protected classifications.
Providers remain responsible for establishing their actual protocol constraints.

`channel.Stream` holds the tune-time URL, classifications and optional expiry.
The URL is deliberately omitted from JSON and formatting/slog representations;
validation rejects non-HTTP/credential-bearing URLs, expired results and invalid
classifications. No stream result is stored as a permanent channel URL.

`provider.Registry` is immutable, rejects duplicate/invalid IDs and defensively
copies metadata. Metadata includes maturity, access type, unofficial mirror and
known capabilities. Stable defaults exclude account-based and unofficial-mirror
sources. Operational enable validation requires an anonymous-free, available,
stable/experimental runtime. Unsupported/account metadata can still be represented
for explicit exclusions. The context-aware runtime interface contains only
Discover and Resolve. Its only implementations are test mocks.

Config adds `[storage] data_dir="./data", state_dir="./state"`; omitted fields
preserve Phase 0 compatibility. Strict key/type validation remains. Structural
config validation never creates files or checks live provider availability;
`ValidateProviders(registry)` is available separately for later operational code.
No portable core code assumes `/etc/arixtv/`.

## Token storage and redacting logging

Tokens contain 32 bytes from `crypto/rand` and encode as `atv_` plus raw URL-base64
(47 characters total). `state/tokens.json` schema version 1 contains only names,
SHA-256 digests and creation/rotation/revocation times. SHA-256 is suitable here
because the input has 256 bits of random entropy, unlike a human password.

Creation/rotation returns a redacting `Secret` wrapper with explicit `Reveal()`.
No read/list API returns plaintext. Formatting, JSON and slog use redacted values.
Verification hashes the candidate once and compares every record's digest with
`crypto/subtle.ConstantTimeCompare`; there is no early match return. Malformed or
corrupt state fails closed. Rotation removes the old digest and rejects accidental
reuse of the same entropy; revoke is idempotent and cannot be undone by rotation.
Mutations take a process lock and atomically replace the full token file. No public
HTTP authentication or lifecycle CLI was added.

`logsafe.New` wraps standard-library slog JSON output, scrubbing sensitive named
attributes/groups recursively. Headers, cookies, tokens, relay secrets and sessions
are redacted. URL credentials, path, every query value and fragment are removed;
relative public/internal bearer paths are scrubbed too. Unknown structs/maps/errors
are suppressed wholesale so their custom methods cannot accidentally serialize
secrets. Safe counts and provider IDs remain visible. Messages/attribute names
must be static, non-secret templates; put opaque secrets in named attributes or
Secret wrappers. No heuristic can identify every unlabeled secret pasted into
ordinary prose. State error summaries use fixed codes instead of upstream text.

## Atomic persistence and Last Known Good behavior

`atomicfile.Write` is the generic byte persistence foundation for future
`playlist.m3u`/`relay.m3u`; no playlist generator was added. `WriteJSON` serializes
before creating the replacement. Each temporary file is in the destination
directory, mode 0600, written completely, synced and closed before atomic rename;
then the directory is synced. Newly created private (0700) directory entries are
synced in their parents. Existing directory permissions are not changed.

Every pre-rename failure preserves the prior file. The post-rename directory-sync
boundary is explicit: `DurabilityError` means replacement occurred but durability
is uncertain. Callers must reload/reconcile rather than assume rollback. A token
mutation at this boundary returns no plaintext; reconcile by name and rotate or
revoke as needed. Guarantees require a trusted local filesystem honoring sync,
atomic rename and advisory locking; NFS/hostile same-user writers are not covered.

`store.Commit` requires an explicitly complete, nonempty discovery candidate,
validates every channel and provider ID, rejects duplicates/stale/inconsistent
values, and atomically writes `data/providers/<provider>.json`. Schema version 1
includes a monotonic generation, attempt/completion timestamps and channels. A
failed/partial/empty/invalid candidate or write error never becomes a replacement.
The future coordinator must mark partial/error discovery incomplete: the store
cannot infer omitted upstream pages hidden behind `Complete=true`.

Existing corrupt or unknown-version state is an error, preserved for operator
recovery. JSON reads reject unknown/duplicate keys, trailing values and excess
size (16 MiB snapshots, 1 MiB token/observation state); no automatic empty-state
reset occurs. No resolved URLs or provider captures are persisted.

Successful status/count/generation are derived from the snapshot itself. Separate
`state/refresh/<provider>.json` observations track attempts and safe failure codes,
without touching LKG channels. Older observations do not mask newer successes.
Health enums can represent healthy, refreshing, failed, stale and unknown; automatic
staleness calculation and aggregation belong to later refresh/status work.

## Refresh locks and platform behavior

`store.LockRefresh(stateDir)` provides the single global scheduled/manual refresh
lock. Linux and Darwin use nonblocking kernel `flock`; contention returns `ErrBusy`.
The caller must hold the lock across the entire future refresh operation. There
is no timer/scheduler/refresh coordinator in Phase 1.

Per-provider mutations and token mutations take separate locks to prevent lost
updates. Persistent lock files are never unlinked, avoiding split-inode locks.
Close or process exit releases the kernel lock; PID timeouts/stale-lock stealing
are unnecessary. Other platforms explicitly return `ErrUnsupported`. A real
subprocess test verifies contention and automatic release after forced process
exit, using pipes for synchronization with no sleeps.

## Dependencies and license status

No production dependency was added. Runtime dependencies remain the Go standard
library and MIT-licensed BurntSushi/toml v1.5.0. Native flock uses standard-library
syscall behind Linux/Darwin build tags. Tool-only govulncheck v1.9.0 is BSD-3-Clause;
its license was inspected and tooling status is documented in the notices file.
ArixTV's own license remains unselected pending owner approval. No FastChannels
source, provider secret or proprietary credential was copied.

## Validation commands and results

All following checks passed using `go version go1.27.3 darwin/arm64`:

```sh
gofmt -w .
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go test -race ./...
go mod tidy
go mod verify
git diff --exit-code -- go.sum
```

All ten internal packages have passing tests, covering channel/metadata JSON,
enums and policy, registry ownership/duplicates, config integration, token entropy,
verification, rotation/revocation, redaction, failed persistence, corrupt-state
handling, atomic-write fault boundaries, LKG preservation, provider isolation,
refresh observations and cross-process locking. Entry point code has no separate
test file; CLI dispatch has existing unit tests and native binary smoke checks.
Normal unit tests make no live internet requests.

Cross-build commands (all packages and CLI, all three targets, CGO disabled):

```sh
for target in linux/amd64 linux/arm64 darwin/arm64; do
  GOTOOLCHAIN=local GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build ./...
  GOTOOLCHAIN=local GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go build -trimpath -o "bin/arixtv-${target%/*}-${target#*/}" ./cmd/arixtv
done
go version -m bin/arixtv-darwin-arm64
```

All six builds passed; embedded compiler metadata confirms Go 1.27.3. Linux
binaries were cross-built, not executed locally. Native macOS lock tests passed;
Ubuntu native tests are configured in CI, whose hosted run awaits a future push.

Scanner and smoke commands:

```sh
GOTOOLCHAIN=local GOBIN="$PWD/bin/tools" go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
GOTOOLCHAIN=local bin/tools/govulncheck -test ./...
bin/arixtv-darwin-arm64 version
bin/arixtv-darwin-arm64 config validate --config config.example.toml
```

The scanner reports **No vulnerabilities found**. The CLI reports development
version metadata and accepts the sanitized example. It still rejects future
`serve` and `refresh` commands. Module tidy did not change production dependencies
or go.sum; module verification passed. Repository hygiene checks include diff and
new-file whitespace checks, private-key/credential/JWT/URL-userinfo pattern scans,
tracked runtime/build-file checks, ignored runtime/config cases, obsolete baseline
search, and confirming the FastChannels mapping has no diff. Build/tool binaries
remain under ignored `bin/`; test state remains in temporary directories.

## Remaining owner review and next phase

No unresolved product-policy decision blocks Phase 2: the owner's mirror,
AES-128, anonymous identity, account and same-egress decisions are now recorded in
the authoritative spec and persistent instructions. ArixTV's own license remains a
distribution/reuse decision; selecting it was not authorized. Individual provider
protocol/permission review remains Phase 3 work, without importing old mapping
exceptions into V1 policy.

Review the schema/version, strict corrupt-state recovery, local-filesystem trust
boundary and post-rename durability-error contract before wiring them into HTTP.
The next planned phase is **Phase 2 — HTTP server and network policy**, after
owner review. It has not started. Real providers remain Phase 3, also unstarted.
