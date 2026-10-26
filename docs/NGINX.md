# ArixTV Nginx Integration

## Design

Use one external domain and one TLS certificate.

Example:

```text
tv.example.com
```

Path routing:

```text
/direct/ → ArixTV
/relay/  → external IPTV relay backend
```

Nginx terminates TLS.

ArixTV remains HTTP-only.

## Direct path

ArixTV natively understands `/direct/` in Phase 2.

Nginx should avoid unnecessary path rewriting.

Example location:

```nginx
location ^~ /direct/ {
    proxy_pass http://127.0.0.1:8787;
    proxy_http_version 1.1;

    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header X-Forwarded-For $remote_addr;

    access_log off;
}
```

The exact generated configuration must be validated against the implemented route behavior.

Do not casually add a trailing URI to `proxy_pass` that changes path semantics.

## Relay path

Conceptual relay block:

```nginx
location ^~ /relay/ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;

    proxy_buffering off;
    proxy_request_buffering off;

    proxy_read_timeout 1h;
    proxy_send_timeout 1h;

    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header X-Forwarded-For $remote_addr;

    access_log off;
}
```

The relay backend should generate URLs containing `/relay/` itself when supported.

Do not use Nginx string substitution to rewrite M3U/HLS payload contents.

## Why path routing

Using paths instead of separate subdomains provides:
- one domain;
- one certificate;
- less DNS maintenance;
- simpler installation instructions.

The limitation is that each backend must correctly support an external path prefix. A relay backend that cannot advertise or preserve a path prefix may require an adapter or may be unsuitable.

ArixTV itself must support `/direct/` natively.

## Forwarded headers

ArixTV trusts forwarded headers only from configured trusted proxy peers.

Typical deployment:

```text
trusted_proxies = ["127.0.0.1/32"]
```

A direct LAN client must not be able to spoof `X-Forwarded-For`.

## Logging and secrets

ArixTV tokens and relay credentials may appear in URL paths or query strings.

Default examples therefore disable access logs for `/direct/` and `/relay/`.

If operators enable logs, use a custom format that does not include full request URI/arguments containing secrets.

Never log real secrets in documentation examples.

## TLS ownership

Nginx/operator owns:
- certificates;
- ACME/certbot/acme.sh;
- TLS protocols;
- HTTP/2 or HTTP/3 decisions.

ArixTV does not own TLS lifecycle.

V1 installer prints configuration; it does not modify Nginx automatically.


## Phase 2 validated forwarding contract

The Direct location above preserves the complete secret-bearing path. Do not add
`/` after the upstream port in `proxy_pass` and do not strip `/direct`. If you
configure a different `remote.direct_path`, use the same location prefix in Nginx;
the server also uses it for LAN Direct. M3U URLs come exclusively from configured
origins, so `Host`, `X-Forwarded-Host`, `Forwarded`, `X-Real-IP` and incoming TLS
claims never become advertised origins.

The supported public proxy convention is exactly one `X-Forwarded-For` IP literal
and `X-Forwarded-Proto: https`, supplied by a configured trusted **socket peer**.
Do not use `$proxy_add_x_forwarded_for` or forward an arbitrary client-supplied
chain. Duplicate/multi-value/malformed trusted proxy values are rejected with 404.
LAN always ignores forwarded identity. If Nginx connects over IPv6 loopback,
configure the corresponding `::1/128` trusted range explicitly. Public tokens
remain mandatory regardless of forwarded metadata. Requests without proxy headers
can be used locally with a token while Remote Direct is enabled; they retain the
socket peer identity and the configured remote origin.

Only the Direct location should reach ArixTV from public traffic. Explicitly keep
local routes outside proxy locations, for example:

```nginx
location = /healthz { return 404; }
location ^~ /internal/ { return 404; }
```

The server independently denies these routes on LAN listeners and rejects
forwarded metadata on local health/internal requests. A proxy that strips all
forwarding information and forwards every path is indistinguishable from a local
client, so restricted Nginx locations remain part of the deployment contract.
`/direct/healthz` is never a health endpoint. `/relay/` remains a future external
backend; Phase 2 does not provide relay.m3u or an IPTV proxy.

Keep `access_log off` as shown. Upstream 302 Location values are signed/provider
URLs and should not be added to logs. The application sets no-store and
no-referrer headers and writes no HTML redirect body containing the upstream URL.
These snippets are documentation only; ArixTV does not modify Nginx or manage TLS.
