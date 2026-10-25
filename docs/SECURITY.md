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
