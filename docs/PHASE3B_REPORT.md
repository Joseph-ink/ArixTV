# Phase 3B completion report — Xumo Play

Phase 3B adds **Xumo (`xumo`) only**. The production registry contains exactly
`vizio` and `xumo`, both **experimental**. Vizio is not promoted; its live playback
remains unverified. No Plex/LG/other provider, Phase 4 coordinator/CLI, scheduler,
installer, EPG/XMLTV, account login or media proxy was implemented. No commit or
push was made.

## 1. Current upstream and independent evidence

Current FastChannels default `development` was rechecked on **2026-10-27
(Asia/Hong_Kong)** at
[`376c1a69b8c11f6271e4557859fee75fe9ec5cb8`](https://github.com/kineticman/FastChannels/tree/376c1a69b8c11f6271e4557859fee75fe9ec5cb8).
Inspected `app/scrapers/xumo.py`, relevant shared `base.py`, `app/routes/play.py`,
`app/worker.py` and README. Identified associated references in
`app/scrapers/category_utils.py`, `app/data/gracenote_map.csv`,
`app/data/gracenote_exclusions.csv` and `app/data/station_index.json`; those
category/guide data are not imported. No upstream implementation or fixtures were
copied or mechanically translated.

Independent [first-party catalog](https://valencia-app-mds.xumo.com/v2/proxy/channels/list/10006.json?sort=hybrid)
requests returned HTTP 200, 232,316 bytes and 454 entries, version 2. Requests
needed no account authentication, fixed geo ID or device query values. Eighteen
callsigns were DRM-marked; the catalog has no explicit positive clear-DRM field.
A CBS News reference was checked through current broadcast → provider-only asset
→ HLS master/media. The final Go live test succeeded without special playback
headers/cookies, keys or segments. The [official website](https://play.xumo.com/)
reported US-only availability through its geo-block page.

Only Xumo's row and its current-evidence section changed in the mapping. Detailed
headers, URLs, assumptions and limitations are in [Xumo notes](providers/XUMO.md).

## 2. Relevant package tree and changed files

```text
internal/provider/
├── builtin/
│   ├── registry.go                    # exactly vizio + xumo
│   └── registry_test.go
├── hlsinspect/
│   ├── inspect.go                     # extracted pure ArixTV HLS inspector
│   └── inspect_test.go
├── vizio/
│   └── hls.go                         # adapters; existing tests unchanged
└── xumo/
    ├── xumo.go                        # lifecycle, metadata, Discover/Resolve
    ├── catalog.go                     # completeness, normalization, fingerprint
    ├── json.go                        # strict bounded protocol envelopes/arrays
    ├── http.go                        # bounded reused transport, safe headers
    ├── resolve.go                     # current asset/source, identities, expiry
    ├── hls.go                         # optional pointer + bounded HLS resolution
    ├── catalog_test.go
    ├── resolve_test.go
    ├── http_test.go
    ├── server_test.go
    ├── test_helpers_test.go
    ├── live_test.go                    # tag AND environment gate
    └── testdata/
        ├── README.md                  # synthetic fixture provenance/review
        ├── catalog.json
        ├── broadcast.json
        ├── asset.json
        ├── master.m3u8
        └── media.m3u8
```

Documentation created: `docs/providers/XUMO.md`, this report.
Updated: `PLANS.md`, `README.md`, `docs/PROVIDERS.md`, Xumo-only mapping evidence,
`docs/ARCHITECTURE.md`, `docs/CONFIGURATION.md`, `config.example.toml` comments, and
`.gitignore` with two exact reviewed Xumo HLS fixture exceptions. No configuration
schema, Channel/Stream/provider interface, snapshot schema, auth/token, HTTP server,
listener, rate-limit or store behavior changed. Go/module/CI files remain unchanged.

## 3. Discovery, completeness and persistence

One anonymous GET to market 10006 with `sort=hybrid`; no geo override, per-channel
manifest checks, image downloads or guide/schedule endpoints. Only stable upstream
numeric-string ID, trimmed name/first genre, US market, classification, last-seen
and fingerprint persist. Language is not guessed; unsupported artwork is omitted.

The complete version-2 `channel.item` envelope is required. Reads are capped at
2 MiB and 2,000 rows, with the row cap enforced while decoding. Missing/malformed
identity/name/callsign, duplicate IDs, empty sets, JSON/type errors, truncation,
size violations, unsupported versions and unknown/duplicate envelope fields return
**nil/error**, never a partial candidate. Any new pagination/count/region envelope
fails closed pending protocol review. Unknown row fields/properties remove Direct
qualification instead of being assumed safe. No opaque payload or program model
is stored. A future coordinator must commit only successful complete discovery.

SHA-256 fingerprints cover normalized metadata, callsign and live/simulcast flags,
excluding volatile last-seen, descriptions, unused fields, URLs and generated IDs.
There are no signed stream URLs, current asset IDs, sessions, cookies, provider
credentials or guide information in Channel snapshots.

## 4. Anonymous identities, headers and region

Market 10006 and API/Origin/Referer values are public route/context conventions,
not credentials; the catalog was independently fetched without authentication.
ArixTV identifies itself as `ArixTV/experimental`. Two independently random
128-bit hexadecimal IDs from `crypto/rand` fill observed device/publisher query
placeholders, only in memory for the provider instance. Limited tracking is 1;
app bundle is ArixTV's own `org.arixtv`. No hardware identifier, extracted secret,
private key, fixed geo ID, account, persistent session or cookie jar is used.
Provider formatting/logging redacts these identifiers.

API Origin/Referer are stripped on CDN/indirection hops, including redirects.
Media is checked without special playback headers. Cookie presence and explicit
IP-binding references are conservatively relay-only. Hidden same-egress binding,
all-country availability and long-term behavior remain unproven. No country/zip,
VPN or source-IP spoofing setting was added.

## 5. Resolve and classification

Each Resolve refetches the catalog, finds the normalized current channel and
rechecks its qualification. It obtains a single unambiguous currently live asset
from the broadcast endpoint, then requests only `f=providers` from the asset API.
It checks current asset timestamps, identity/type, provider availability and HLS
format. It does not fall back to a scheduled/non-live or arbitrary first asset,
read an EPG, cache an asset or persist a final URL.

Direct qualification is deliberately narrow: only observed CBS News 24/7 ID
`99991158`, callsign `XCBSNWS-CMS`, expected normalized name, live/simulcast flags
and understood metadata. Other non-DRM entries are **unknown**, not presumed clear.
DRM suffixes become `other_drm` / `drm`; non-live entries are unsupported. All
unknown/protected/unsupported snapshot entries stay out of public Direct M3U.

At tune time, unknown/protected source fields, unsafe URLs, unsupported formats or
manifest protection fail closed. Cookie-setting responses or explicit IP binding
produce `clear_relay_only`; the approved public server refuses those results.
Ordinary identity-key AES-128 is separately `hls_standard_encrypted`, without key
retrieval. No DRM handling or bypass was added.

The returned stream is a fresh validated media-manifest URL. One optional JSON
string pointer (8 KiB) may precede at most a master and one muxed media manifest
(256 KiB each). Deeper nesting and separate audio/video fail closed; optional
subtitles and adaptive bitrate switching are omitted. There is no manifest
rewriting, key/segment request, streaming or video buffering.

Recognized URL expiry, live asset end, provider sunset and the API refresh hint's
future upper bound limit the result. These are conservative ceilings, not a promise
that the CDN URL lasts until that time. Actual URL lifetime and cross-egress
reliability still require later testing.

## 6. Two protocol findings corrected with offline regressions

1. Live API responses can contain a future `refresh.from/to` range with an
   already-live asset. The initial implementation incorrectly treated refresh
   as the content start window. It now validates the ordered freshness hint and
   future ceiling while checking availability against the asset timestamps.
2. The source URL contains an opaque ad-routing value with a literal semicolon.
   Full `url.ParseQuery` rejected it during expiry checking. Xumo now scans only
   recognized expiry fields, retaining other query bytes unchanged; malformed or
   duplicate actual expiry fields remain rejected.

Both are Xumo-local interpretations, with synthetic deterministic regression tests.
Neither changes Vizio behavior or the generic provider interface. No new signing
secret or generic contract limitation was found.

## 7. Shared HLS inspector and Vizio impact

Moved the existing ArixTV pure inspector into `provider/hlsinspect`, avoiding two
divergent HLS parsers. It receives an absolute-URL policy and performs no I/O.
Vizio keeps thin adapters preserving its original errors, URI policy and bounded
first-muxed-variant behavior. Its existing tests are unchanged and pass, including
the real-runtime mocked server 302 compatibility test. Its networking, catalog,
metadata, maturity and resolver policy are unchanged. No Vizio live debugging or
new Vizio support was undertaken.

## 8. Offline end-to-end compatibility and resource limits

The automated test proves:

```text
actual Xumo runtime + mocked TLS endpoints
→ complete Discover
→ temporary schema-1 snapshot commit
→ existing server catalog Reload
→ protected M3U with qualified Xumo channel
→ /direct/s/<token>/play/xumo/99991158
→ fresh Xumo Resolve
→ zero-body HTTP 302
```

The same test also includes a real Vizio runtime/identity in the registry/catalog,
without contacting Vizio live. Vizio's unchanged separate runtime/server test
continues to exercise its resolver. Production-registry tests assert exactly the
two experimental anonymous/free runtimes and reject every unapproved provider.
Sanitized failure responses and absence of URL/identity/guide leakage are checked.

Networking uses one reused standard-library transport, normal TLS verification,
public HTTPS/443 and checked resolved IPs, bounded redirects/headers/bodies,
context cancellation and no environment proxy/cookie jar/retries. Each operation
has an eight-second deadline and two active provider slots with no queue.
Resolve needs three metadata GETs and at most three pointer/manifest GETs, all
within the same deadline. There is no unbounded goroutine fan-out or scheduler.
Normal tests use local TLS servers, temp state and synthetic fixtures, never live
internet. Test coverage includes invalid/incomplete discovery, identity, unknown
schema/classification, body/time/concurrency bounds, fresh lookup, expiry,
HLS/pointers, cookies/IP binding, redirects/destinations, TLS rejection, API-header
isolation, formatting/log secrecy and serialization omissions.

## 9. Live results, separately from offline tests

The opt-in live command requires both gate mechanisms:

```sh
GOTOOLCHAIN=local ARIXTV_XUMO_LIVE=1 go test -tags=integration \
  ./internal/provider/xumo -run '^TestLiveXumo$' -count=1 -v
```

Two initial smoke attempts failed on the refresh-hint and opaque-query checks
above. Independent bounded observations, an offline replay and one bounded
network trace isolated them. The temporary diagnostic code was removed; raw
research material stayed outside the repo. No retry loop or media/key fetch ran.

The final corrected smoke **passed in 1.48 seconds**:

```text
454 catalog channels
1 evidence-qualified Direct identity
18 DRM-marked entries
435 other/unknown (including non-live)
fresh HLS media manifest validated without special playback headers or cookies
```

This proves one sampled discovery/Resolve sequence on one egress. It does not
establish multiple channels, repeated reliability, actual URL expiry, all regions
or absence of hidden IP binding. Xumo remains experimental. Vizio remains
experimental with live playback unverified. A tag-only test run without the
environment variable skips and is not counted as live success.

## 10. Dependencies, licensing and validation

No new production, test or tooling dependency. Go standard library only for Xumo
and the HLS extraction. BurntSushi/toml v1.5.0 (MIT) remains the sole runtime module;
`THIRD_PARTY_NOTICES.md`, `go.mod`, `go.sum` and pinned CI scanner configuration are
unchanged. No upstream license was assumed and no upstream code was bundled.
ArixTV's license and provider service/redistribution terms remain owner review
items before distribution or stable promotion.

Native compiler: `go version go1.27.3 darwin/arm64`. Exact required local checks:

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
go test -cover ./internal/provider/...
bin/tools/govulncheck -version
bin/tools/govulncheck -test ./...
go run ./cmd/arixtv config validate --config config.example.toml
go test -tags=integration ./internal/provider/xumo -run '^TestLiveXumo$' -count=1 -v
```

Formatting, vet, all 16 internal package tests/race checks, module verification and
example validation pass. Provider coverage: Xumo **90.1%**, shared HLS **99.0%**,
Vizio **84.2%**, registry **97.6%**, builtin wiring **75.0%**. The final tag-only run
skips live work without its environment gate. Govulncheck uses official pinned
v1.9.0 and Go 1.27.3, with the intentionally live database (updated
2026-10-21 00:27:29 UTC): **No vulnerabilities found**.

All three cross-builds passed. Commands, CGO disabled:

```sh
for target in linux/amd64 linux/arm64 darwin/arm64; do
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build ./...
  GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 \
    go build -trimpath -o "bin/arixtv-${target%/*}-${target#*/}" ./cmd/arixtv
done
```

Linux binaries are cross-built, not executed locally. Binaries remain in ignored
`bin/`; hosted CI was not dispatched because no push was requested. Hygiene checks
include whitespace, private-key/PAT/AWS/JWT/literal-token/URL-userinfo patterns,
reviewed fixtures, ignored runtime paths and unchanged approved core/module/CI
files. Only the two exact reviewed synthetic Xumo HLS files bypass playlist ignore
rules; no real provider sessions, credentials, signed captures or runtime state
are included.

## 11. Owner review and next provider

No secret/authentication or architecture blocker prevents this experimental
reference subset. Review the intentional one-identity Direct profile before any
broader Xumo qualification. Broader channel validation, repeated tuning, regions,
cross-egress behavior, actual URL lifetime and provider terms remain uncertain.
Both providers must stay experimental until the owner's later promotion review.

Recommend **Plex as the next research candidate**, based on the existing mapping's
anonymous/public discovery and tune-time flow. Its current protocol and rights
must be reverified only after owner approval. No Plex implementation or new
provider research was started in this phase. Phase 3B stops after Xumo; Phase 4
and installer work remain unimplemented pending review.
