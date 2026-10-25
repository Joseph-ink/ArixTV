# Phase 0 completion report

Completed 2026-10-26 (Asia/Hong_Kong). Active project: `/Users/joseph/ArixTV`.
Phase 1 is unstarted and requires owner review before continuing.

## Repository tree

```text
ArixTV/
├── .github/workflows/ci.yml
├── .gitignore
├── AGENTS.md
├── ARIXTV_SPEC.md
├── CODEX_START.md
├── PLANS.md
├── README.md
├── THIRD_PARTY_NOTICES.md
├── cmd/arixtv/main.go
├── config.example.toml
├── deploy/linux/README.md
├── docs/
│   ├── ARCHITECTURE.md
│   ├── CONFIGURATION.md
│   ├── FASTCHANNELS_MAPPING.md
│   ├── INSTALL.md
│   ├── NGINX.md
│   ├── PHASE0_REPORT.md
│   ├── PROVIDERS.md
│   └── SECURITY.md
├── go.mod
├── go.sum
└── internal/
    ├── cli/
    │   ├── cli.go
    │   └── cli_test.go
    └── config/
        ├── config.go
        ├── config_test.go
        └── validate.go
```

Ignored local files include `.DS_Store` and `bin/` build outputs. Git metadata
is omitted above. Runtime data, provider snapshots and playlists were not created.
The upstream research checkout is outside the repository.

## Created and changed files

Created: `.github/workflows/ci.yml`, `.gitignore`, `go.mod`, `go.sum`,
`cmd/arixtv/main.go`, `internal/cli/cli.go`, `internal/cli/cli_test.go`,
`internal/config/config.go`, `internal/config/validate.go`,
`internal/config/config_test.go`, `config.example.toml`,
`deploy/linux/README.md`, `docs/CONFIGURATION.md`, `THIRD_PARTY_NOTICES.md`,
and this report.

Changed existing documents: `PLANS.md`, `README.md`, `docs/ARCHITECTURE.md`,
`docs/PROVIDERS.md`, and `docs/FASTCHANNELS_MAPPING.md`.

Read and preserved: `AGENTS.md`, `ARIXTV_SPEC.md`, `CODEX_START.md`,
`docs/INSTALL.md`, `docs/NGINX.md`, and `docs/SECURITY.md`.

There were no commits when work started; supplied repository documents were
untracked. No files were staged, committed, or pushed during this task.

## Validation commands and results

| Command / check | Result |
|---|---|
| `gofmt -w .` | Passed; source formatted |
| `test -z "$(gofmt -l .)"` | Passed; no unformatted Go files |
| `go vet ./...` | Passed on Go 1.27.3 |
| `go test ./...` | Passed: CLI and configuration tests; entry point has no separate test file |
| `go mod tidy` and `go mod verify` | Passed; dependency graph clean and checksums verified |
| `GOTOOLCHAIN=go1.25.0 go vet ./...` | Passed on declared minimum toolchain |
| `GOTOOLCHAIN=go1.25.0 go test ./...` | Passed on declared minimum toolchain |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o bin/arixtv-linux-amd64 ./cmd/arixtv` | Passed; ELF x86-64, statically linked |
| `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/arixtv-linux-arm64 ./cmd/arixtv` | Passed; ELF aarch64, statically linked |
| `GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -o bin/arixtv-darwin-arm64 ./cmd/arixtv` | Passed; Mach-O arm64 |
| All three build commands above with `GOTOOLCHAIN=go1.25.0` | Passed; matches CI's Go version |
| `bin/arixtv-darwin-arm64 version` | Passed; development metadata |
| `bin/arixtv-darwin-arm64 config validate --config config.example.toml` | Passed; exit 0 |
| Native CLI with nonexistent config | Passed; exit 1; no path/value echo |
| Native CLI with `serve` | Passed; exit 2; later-phase command unavailable |
| `go build -o bin/arixtv-metadata -ldflags '-X main.version=v0.0.0-test -X main.commit=test-commit -X main.buildDate=test-date' ./cmd/arixtv` then `bin/arixtv-metadata version` | Passed; injected metadata displayed |
| Inventory enumeration against inspected upstream | Passed; all 52 built-in IDs represented; dynamic family explicit |
| Mapping source-link validation | Passed; all 96 linked upstream file paths exist at inspected commit |
| Secret-pattern scan of all includable files | No non-placeholder credential/key/JWT/URL-userinfo matches; examples reviewed |
| `git diff --check` and per-new-file `git diff --no-index --check /dev/null FILE` | Passed after removing an extra EOF blank line |
| `git check-ignore` runtime/config cases | Passed for local configs, data, state, captures, private data, playlists and binaries; example/test files remain includable |
| `git ls-files` runtime/build patterns | No generated/runtime files tracked |

Tests cover defaults, TOML syntax/types/unknown keys, IPv4/IPv6 and CIDR checks,
loopback/wildcard constraints, HTTPS origin/path constraints, durations,
concurrency, provider-ID syntax, file/input failures, size limits, diagnostic
non-disclosure, command dispatch, exit codes and local default config behavior.
Fuzz seeds run in normal tests; no extended fuzz campaign is claimed.

GitHub Actions was added with formatting, vet, tests, module verification,
example validation and all three build targets. Its checks passed locally;
hosted Actions execution has not occurred because no push was requested.
Linux binaries were cross-built, not run on a Linux host.

## Upstream findings and V1 selection

The current FastChannels default branch `development` was inspected at commit
`7e61c1ef127f7119326eb98381726d6f7cfaa56d` (release subject 5.7.2).
The mapping includes 52 built-in source identities, one dynamic playlist source
family, and 35 expanded Free Networks adapters. Every source records access,
discovery, resolution, stream/DRM/delivery classification, region/session limits,
V1/future/exclusion decision, implementation status, and immutable evidence.

**Vizio, Xumo and Plex** appear best suited for initial Phase 3 protocol work;
**LG** is a reserve. Narrow free feeds such as Bally Sports Live, Hallmark,
DW, NHK, CBN and The CW are additional candidates. No live playback was tested,
so these remain research decisions, not support claims.

STIRR uses proxy routes for IP-bound SSAI; Zeam documents IP/UA binding; Samsung
depends on a third-party catalog mirror; VIDAA requires app-signed backend
requests. Paid/account sources are explicitly retained as exclusions. Conflicts
between upstream source notes and resolver code are documented, including TVE
sources, DAZN entitlements and Tennis Channel protection.

No upstream provider code was copied or translated. No provider implementation,
EPG/XMLTV subsystem, media relay, DRM support, browser runtime, UI, database,
built-in TLS, or Linux service management was added.

## Owner review and next phase

Review before later provider work:

- ArixTV's own license remains undecided. No upstream root license was found;
  obtain permission/license clarity before any FastChannels code reuse.
- The only added dependency is MIT-licensed BurntSushi/toml v1.5.0; its notice
  is preserved in `THIRD_PARTY_NOTICES.md` for future distributions.
- Decide whether third-party channel mirrors are acceptable, how ordinary
  AES-128 HLS fits the clear-stream policy, and which anonymous public-app
  identities/signing mechanisms are permitted without copied credentials.
- Resolve preview/free-account intent and same-egress-only compatibility before
  accepting affected providers. These are not Phase 0 blockers.

The next planned phase is **Phase 1 — Core domain and state**: portable channel,
stream/provider models, registry, config integration, secret-redacting logging,
hashed token storage, atomic snapshots/playlists, last-known-good behavior and
refresh locking. It has not been started.
