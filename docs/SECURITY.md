# ArixTV Security Model

## Security goals

ArixTV aims to protect:
- private playlist access;
- tune-time resolver access;
- provider session data;
- internal relay resolver access;
- local configuration;
- logs.

ArixTV does not rely on obscurity of endpoint paths as a security control.

## Public bearer tokens

Public playlist/play access uses strong random bearer tokens.

Requirements:
- generated using cryptographically secure randomness;
- target entropy: 256 bits;
- persisted as hashes where practical;
- cleartext shown only when created/rotated;
- full token never logged.

A leaked bearer token grants the access associated with that token until revoked.

Operators should create separate tokens for separate devices/users when useful.

## Unauthorized behavior

For public endpoints, invalid/missing tokens should normally receive:

```text
404
```

This minimizes endpoint/auth state disclosure.

## LAN mode

LAN mode is not "no authentication."

A request must satisfy:

```text
source address is in configured allowed CIDR
AND
token is valid
```

Prefer explicit LAN address binding over `0.0.0.0`.

## Reverse proxy mode

Remote Direct traffic:

```text
client HTTPS → Nginx → ArixTV loopback HTTP
```

ArixTV trusts forwarded headers only if the actual socket peer is configured as a trusted proxy.

Never trust `X-Forwarded-For` from arbitrary clients.

## Internal relay resolver

The relay resolver:
- is separate from public routes;
- uses a separate internal secret;
- is loopback-only by listener/policy;
- must not be exposed through LAN/public routing.

## Secret logging

Never log:
- public tokens;
- internal relay secrets;
- provider cookies;
- Authorization headers;
- provider bearer tokens;
- private keys;
- signed URL secrets.

HTTP request logging must redact protected path components.

Nginx examples default to `access_log off` on secret-bearing IPTV routes.

## Repository secrets

Never commit:
- `.env` with secrets;
- real `config.toml`;
- generated runtime state;
- generated M3U with real tokens;
- private provider captures;
- private keys;
- API credentials.

Use placeholders in documentation.

## HTTP server hardening

Set explicit:
- header read timeout;
- read timeout;
- write timeout;
- idle timeout;
- max header bytes.

Public resolver routes accept only needed methods, normally `GET` and `HEAD`.

Use bounded rate limiting and bounded provider resolution work.

## Token brute force

With properly generated 256-bit tokens, brute-force discovery is infeasible.

Rate limiting still matters for:
- resource exhaustion;
- scanning;
- provider amplification;
- denial of service.

## TLS

ArixTV does not manage TLS.

Remote public deployments should normally use Nginx HTTPS.

LAN deployments may use HTTP when the local network is considered trusted enough for the operator's threat model.

The ArixTV core remains HTTP-only to keep certificate lifecycle separate.

## DRM boundary

ArixTV does not bypass DRM.

Security/research work may identify protected streams only to classify them unsupported.

Do not implement:
- decryption;
- key extraction;
- license circumvention;
- protected stream conversion.

## Supply chain

Release/install/update workflow should:
- use GitHub Releases;
- publish SHA256 checksums;
- verify downloaded assets before installation;
- preserve previous binary for rollback.

Optional relay binaries should be version-pinned and checksum-verified.

## System privileges

Linux daemon should run under dedicated non-root `arixtv` user.

Use systemd hardening that does not prevent required network access or state writes.

Keep writable paths limited to ArixTV state directories.

## Reporting security issues

Before public release, define a responsible disclosure contact/process in the repository.


## Upstream source-code boundary

FastChannels is the primary provider research reference, but it is not a source-code dependency.

Do not mechanically translate or copy its implementation where reuse rights are unclear.

Keep ArixTV provider code independently implemented and keep third-party license review explicit.

## Approved owner policies (Phase 1)

V1 is anonymous/free FAST only. Account login, including nominally free accounts,
is excluded. Provider-owned public APIs are preferred; unofficial mirrors remain
experimental/future by default. Anonymous protocols may use public non-secret
constants/client IDs, independently understood algorithms and locally generated
session IDs. Embedded app secrets, private keys, extracted authentication secrets
and copied repository secrets must never be distributed; an indispensable secret
makes the provider unsupported pending explicit owner reconsideration.

Ordinary HLS AES-128 is represented as `hls_standard_encrypted`, distinct from
licensed DRM. Normal client HLS key retrieval may be in scope. Special header,
cookie, identity or same-egress/session constraints mean `clear_relay_only` when
they prevent normal direct playback. Such channels are excluded from both LAN and
Remote Direct output. Widevine, PlayReady, FairPlay, other protected DRM, browser
bridges and license-system circumvention remain out of scope. There is no DRM
implementation in the model; it only supports classification/exclusion.

## Implemented Phase 1 token and logging foundations

Public secrets have 256 bits from `crypto/rand`, encoded as `atv_` plus 43 raw
URL-base64 characters. Only SHA-256 digests, names and lifecycle timestamps are
stored in version-1 token JSON. High-entropy random tokens do not need password
stretching or a recoverable encryption key. Verification computes one digest and
performs constant-time comparisons across every record. Revoked digests never
match; rotation removes the previous digest. Corrupt state errors fail closed.

Creation/rotation returns a `Secret` wrapper; its formatting, JSON and slog
representations redact. `Reveal()` is explicit and must be used only to deliver
the new token to the operator. Neither listing nor reading state returns plaintext.
Keep token names descriptive and non-secret. Secret strings, once revealed, cannot
be automatically erased from the Go heap; do not retain or log them. Lifecycle
CLI commands and HTTP authorization remain later work.

`logsafe.New` uses the standard-library JSON slog handler behind sanitization:
sensitive attributes/groups (Authorization, cookies, tokens, secrets, sessions,
passwords, signatures, API keys and headers) are redacted recursively. URL user
info, paths, all query values and fragments are removed; known ArixTV tokens and
relative public/internal secret path segments are scrubbed. Arbitrary maps,
structs and errors are redacted wholesale instead of calling potentially unsafe
formatting/serialization methods. Safe counts/provider IDs remain useful.

Logging messages and attribute names must be static, non-secret templates. Put
sensitive values in appropriately named attributes or use `Secret`, never build
free-form messages containing arbitrary secrets. No heuristic scrubber can infer
an unlabeled opaque secret pasted into ordinary prose. State error summaries are
fixed safe failure codes, not upstream error bodies or signed URLs. Tests exercise
nested/bound attributes, groups, headers, errors, URLs and relay/public path tokens.

## Filesystem trust and durability

Stores receive configured roots and use temporary directories in tests. Newly
created directories use 0700; replaced files and lock files use 0600. Existing
operator-owned directory permissions are not silently changed. Parents must be
trusted local paths, protected against other users; these foundations do not
attempt to defend against a malicious same-user process changing symlinks/files.
Advisory locks require all writers to cooperate. NFS/network-filesystem locking,
rename and sync semantics have not been certified.

Each temporary file is synced/closed before same-directory atomic rename, then
the directory is synced. Newly created directory entries are also synced in their
parents. All pre-rename failure cases preserve the old file. If the post-rename
sync fails, `atomicfile.DurabilityError` explicitly reports that the new state may
already be active. Reload it; never assume old token/snapshot validity or attempt
a blind rollback. If a token mutation hits this boundary, no new plaintext is
returned: reconcile by name and rotate/revoke as needed. Power-loss guarantees
ultimately depend on filesystem/storage honoring successful sync calls.

Snapshot schema validation and last-known-good rules reject incomplete, empty,
duplicate, inconsistent, protected-as-clear, stale, malformed and oversized
candidates. Unknown/corrupt existing state is preserved for explicit recovery.
There is no automatic downgrade to empty state. Kernel refresh locks release on
process exit without deleting their persistent inode or using unsafe PID stealing.

## Go toolchain and vulnerability scanning

Minimum support, CI and future release builds use **Go 1.27.3**. Older baseline
compatibility has been removed. The compiler includes the crypto/x509 mitigation
for CVE-2026-111943, relevant to future outbound provider HTTPS; see the
[official release history](https://go.dev/doc/devel/release#go1.27.3).
CI sets `GOTOOLCHAIN=local` so dependencies cannot silently select another compiler.
Future Phase 8 release automation must follow the same security baseline.

A separate CI job installs the official scanner with the exact command:

```sh
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
govulncheck -test ./...
```

Scanner version v1.9.0 is fixed in the workflow and verified through normal Go
module checksum verification. It is BSD-3-Clause licensed and development-only,
with its versioned transitive dependencies outside the production module graph.
The current vulnerability database is intentionally live, so findings may change
without code changes. This is a reproducible scanner configuration, not an
immutable database snapshot. A network/scanner failure fails the job. Normal unit
tests have no network dependency. See the [Go vulnerability management guide](https://go.dev/doc/security/vuln/).
