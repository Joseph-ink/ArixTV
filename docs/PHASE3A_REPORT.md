# Phase 3A completion report — Vizio reference provider

Phase 3A implements **Vizio WatchFree+ (`vizio`) only**. Phase 2's approved HTTP,
network, authentication, LKG/reload and schema contracts are unchanged. No second
provider, refresh coordinator/CLI, scheduler, installer, unit file, EPG, XMLTV,
media proxy, DRM handler or account login was added. No commit or push was made.

## Current protocol research

Inspected FastChannels current default `development` at commit
[`40f5903cb43979e00e8d6cc7186d855c93eed6b1`](https://github.com/kineticman/FastChannels/tree/40f5903cb43979e00e8d6cc7186d855c93eed6b1)
on 2026-10-26: `app/scrapers/vizio.py`, `app/scrapers/base.py` (`resolve`) and
README Sources / Source Notes. Its Vizio path still expands catalog URLs during
discovery and returns stored URLs unchanged at tune time. No upstream source or
tests were copied or translated. No root license was found in the research checkout.

Independent requests to the provider-owned
[public catalog](https://watchfreeplus-epg-prod.smartcasttv.com/api/channels)
returned HTTP 200, 2,328,281 bytes, 472 channels and explicit protection/mobile
eligibility fields. One NFL entry had both a license URL and a token endpoint;
no protected endpoint was contacted. The Vizio row alone was updated in the
historical migration inventory, with a separate current evidence section.

[Provider notes](providers/VIZIO.md) record discovery/resolution, headers, region,
sessions, formats, lifetime uncertainty, rate limits and instability. Those notes
distinguish source evidence, direct API observations and inferred compatibility.

## Relevant tree and changed files

```text
internal/
├── cli/cli.go                         # production registry wiring
└── provider/
    ├── registry.go                    # comments updated, contract unchanged
    ├── builtin/
    │   ├── registry.go                # only Vizio
    │   └── registry_test.go
    └── vizio/
        ├── vizio.go                   # Discover / Resolve / metadata
        ├── catalog.go                 # bounded decode / normalization / fingerprint
        ├── http.go                    # reused transport / safe requests
        ├── hls.go                     # bounded manifest inspection, expiry
        ├── vizio_test.go
        ├── hls_test.go
        ├── server_test.go             # offline real-runtime server compatibility
        ├── live_test.go               # integration tag + explicit environment gate
        └── testdata/
            ├── README.md             # synthetic fixture provenance/review
            ├── catalog.json
            ├── master.m3u8
            └── media.m3u8
```

New documentation: `docs/providers/VIZIO.md`, this report.
Updated: `.gitignore` (two exact synthetic HLS fixture exceptions), `PLANS.md`,
`README.md`, `config.example.toml` (comments only), `docs/PROVIDERS.md`,
`docs/FASTCHANNELS_MAPPING.md`, `docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`,
`internal/cli/cli.go`, and `internal/provider/registry.go` (comments only).
The authoritative product spec and Phase 1/2 implementation foundations were not
changed. Config schema, snapshot version, CI and dependency graph are unchanged.

## Discovery, completeness and persisted metadata

One catalog GET; no per-channel manifest probes, EPG calls or image downloads.
Typed decoding drops unused fields. At most 4 MiB and 2,000 rows are accepted,
with the row cap enforced while decoding. Results are sorted by stable upstream ID,
with trimmed display/group, safe public artwork, US market and no guessed language.

Any malformed/missing required channel identity/name/URL array, duplicate ID,
invalid date/type, missing/empty set, oversize response, duplicate/unknown envelope,
continuation flag or inconsistent total/ID set returns **nil/error**. Partial data
cannot accidentally look like successful discovery. Unknown envelope changes
require protocol review rather than risking a truncated replacement. The later
coordinator must commit only a successful complete result; it is not implemented.

Persisted values contain only normalized Channel fields, last-seen and a SHA-256
fingerprint. Fingerprints cover metadata/classification and delivery origin/path,
excluding volatile query values, last-seen and unused provider fields. No URLs for
playback, signed credentials, cookies, sessions, opaque provider payloads or guide
fields enter snapshots. Ephemeral playback data exists only during the call.

## Resolution and classification

Resolve fetches the current catalog and looks up the upstream channel ID. It
rechecks current eligibility/protection, expands recognized public query macros
using ArixTV's own anonymous identity/privacy defaults and preserves untouched
signed values. Unknown macros fail closed. No app impersonation, copied secret,
account bootstrap, token endpoint or license exchange is used.

Clear Direct candidates need explicit empty license/token fields, FAST/mobile
eligibility, active supplied windows and supported HLS/HTTPS references. Licensed
entries are `other_drm` / `drm`; token-gated entries are unsupported. Missing
protection fields, uncertain states/formats and unknown macros remain unknown.
This is experimental metadata qualification, not all-channel playback proof.

Tune validation fetches at most two 256 KiB manifests: direct media or one master
and its first muxed media variant. The returned URL is that validated media URL;
there is no rewriting or media/key fetching. Separate audio/video, deeper nesting,
variables, malformed manifests and protected/unknown encryption fail closed.
Optional subtitles are omitted. AES-128 with an ordinary identity key URI is
`hls_standard_encrypted`, without fetching the key. Cookie-setting responses and
explicit IP-binding references are conservatively relay-only; the public server
rejects them. Both Direct and relay-only classifications fit the existing provider
boundary; the CLI internal relay validator remains disabled by default.

Known `exp`/`expires` timestamps must be valid/future, with earliest expiry retained.
Other URL lifetimes and hidden IP binding remain unknown; no lifetime soak test
or cross-egress certification was performed. Fixed errors never expose URLs,
headers, cookies, raw response bodies or transport/parser details.

## Networking, region/session behavior and registry

Eight-second whole-operation context deadline, two concurrent provider operations,
no queue/retry loop, reused standard-library transport/keep-alive, bounded headers,
bodies/connections and redirects. Normal TLS verification remains enabled. Only
public HTTPS/443 destinations are accepted, with resolved-IP checks on production
dials; downgrade/private redirects are denied. No environment proxy or cookie jar.
At most two redirects per GET; no automatic re-authentication or session storage.
No new configuration is needed: this is the anonymous US mobile catalog. No
country/zip spoofing is implemented. API availability does not establish every
CDN's country/egress compatibility; quotas/stability guarantees are unestablished.

Production `builtin.NewRegistry` installs only Vizio, marked `experimental`,
`anonymous_free`, `unofficial_mirror=false`. Metadata expresses clear Direct,
relay-only, DRM, unsupported and unknown capabilities. Constructors do no network
I/O. `providers.enabled=["vizio"]` is accepted by serve; the default remains empty.
Serving consumes existing snapshots and does not initiate discovery/refresh.

## Offline and opt-in integration tests

Fifteen Vizio test functions plus subtests cover discovery/normalization/IDs,
serialization omissions, malformed JSON/types/status/size, incomplete catalogs,
duplicates, fingerprints, current metadata rechecks, fresh resolution, expiry,
DRM/AES-128, HLS grammar/subtitles/rendition bounds, cookie/IP classification,
redirect/destination restrictions, cancellation/deadlines/capacity and secret-safe
errors/logs. Fixtures are hand-authored and reviewed, with fake example domains
and deterministic signatures; no live captures were copied.

The offline compatibility test proves:

```text
actual Vizio runtime + mocked TLS API/HLS
→ Discover → committed temporary schema-1 snapshot
→ existing server catalog → token-protected Direct M3U
→ /direct/s/<token>/play/vizio/news-1
→ current Vizio Resolve → zero-body HTTP 302
```

No upstream URL or token is written into snapshot/log output. The existing public
playlist excludes protected and unknown Vizio entries. Normal tests make no live
internet requests. The opt-in test is absent without `integration`; even with that
tag it skips unless `ARIXTV_VIZIO_LIVE=1` is explicitly provided.

## Live results — not a passing certification

Independent low-volume, queryless sample probes: one HLS master HTTP 200, one
HTTP 400, one HTTP 403; these are not normal playback certification.

This opt-in command was run twice, with no retry loop:

```sh
ARIXTV_VIZIO_LIVE=1 go test -tags=integration ./internal/provider/vizio \
  -run '^TestLiveVizio$' -count=1 -v
```

1. Discovery succeeded: 472 channels, 429 catalog-qualified Direct candidates,
   one protected, 42 other/unknown. Resolve returned an HLS-inspector error after
   the selected master included optional subtitles. That overly restrictive case
   was corrected and covered by an offline regression test.
2. The re-run timed out during catalog retrieval under the eight-second operation
   deadline. No live Resolve success was obtained.

Therefore **live tune-time playback remains unverified**. This external/protocol
validation limit is separate from the successful deterministic tests below.
Privacy defaults, CDN failures, region, cross-egress behavior, URL expiry and
latency require further low-volume testing before stable promotion.

## Dependencies and licensing

**No new production or test dependency.** Standard library only for Vizio.
BurntSushi/toml v1.5.0 (MIT) remains the sole production module dependency;
`THIRD_PARTY_NOTICES.md`, `go.mod` and `go.sum` are unchanged. Existing official
`govulncheck@v1.9.0` remains development-only, with its intentionally live database.
No FastChannels code/fixture is distributed and no upstream reuse license was
assumed. Owner selection of ArixTV's own license and provider service terms review
remain unresolved before distribution/stable support.

## Exact validation commands and results

Native toolchain: `go version go1.27.3 darwin/arm64`. Required local checks pass:

```sh
export GOTOOLCHAIN=local
gofmt -w .
test -z "$(gofmt -l .)"
go mod tidy
go mod verify
git diff --exit-code -- go.mod go.sum
go vet ./...
go test ./...
go test -race ./...
bin/tools/govulncheck -version
bin/tools/govulncheck -test ./...
go run ./cmd/arixtv config validate --config config.example.toml
```

All 14 internal packages pass tests/race; entry point compiles. Modules verified,
no module-file diff, no scanner vulnerabilities. The scanner identifies
`govulncheck@v1.9.0`, Go 1.27.3 and database update 2026-10-21 00:27:29 UTC.
An additional provider coverage run passed (Vizio approximately 86%, builtin 100%):

```sh
go test -cover ./internal/provider/...
go test -tags=integration ./internal/provider/vizio -run '^TestLiveVizio$' -count=1 -v
```

The second command intentionally **skips** live work without the environment gate;
it does not stand in for a successful live smoke.

All target builds pass, CGO disabled, with Go 1.27.3:

```sh
for target in linux/amd64 linux/arm64 darwin/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build ./...
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go build -trimpath -o "bin/arixtv-${target%/*}-${target#*/}" ./cmd/arixtv
done
```

Linux targets are cross-built, not executed locally. Hosted CI was not dispatched
(no push). Diff whitespace, private-key/PAT/AWS/JWT/literal-token/URL-userinfo scans,
fixture review and ignored-path checks pass: only synthetic invalid URL-userinfo
examples in tests matched; no secrets or runtime/generated state are included.
Only two exact reviewed HLS fixture paths bypass the generated-playlist ignore
rule. Required Go baseline, Phase 1/2 foundations and module/CI files remain intact.

## Review limits and next provider recommendation

No secret, account login or DRM protocol is needed for the implemented anonymous
subset. No implementation dependency/legal reuse blocker was discovered. Stable
promotion is blocked by incomplete live validation and unresolved provider terms/
distribution licensing, not by offline test failures.

Review the deliberate bounded first-muxed-variant behavior (no ABR/subtitles),
conservative catalog qualification and fresh catalog-per-tune cost. These are
provider-local choices, with no redesign of approved serving/state contracts.

Recommend **Xumo as the next research candidate**, based on the existing inventory;
its current protocol, clear subset and rights must be reverified after owner
approval. No Xumo code/research implementation was started here. Phase 3A stops
at Vizio; broader Phase 3 and Phase 4 remain unimplemented pending review.
