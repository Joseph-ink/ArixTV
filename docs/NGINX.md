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

ArixTV should natively understand `/direct/`.

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
