package vizio

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Fixed errors deliberately discard URL-bearing net/http errors and upstream bodies.
var (
	ErrNetwork     = errors.New("vizio: network request failed")
	ErrStatus      = errors.New("vizio: upstream status rejected")
	ErrResponse    = errors.New("vizio: invalid or oversized response")
	ErrCatalog     = errors.New("vizio: incomplete or invalid catalog")
	ErrUnavailable = errors.New("vizio: channel unavailable for anonymous direct playback")
	ErrManifest    = errors.New("vizio: unsupported or invalid HLS manifest")
	ErrBusy        = errors.New("vizio: request capacity reached")
)

const operationTimeout = 8 * time.Second

func publicURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || len(raw) > 8192 || strings.ContainsAny(raw, "\r\n\\") {
		return nil, ErrUnavailable
	}
	if u.Port() != "" && u.Port() != "443" {
		return nil, ErrUnavailable
	}
	if strings.EqualFold(u.Hostname(), "localhost") || strings.Contains(u.Hostname(), "%") {
		return nil, ErrUnavailable
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !publicIP(ip) {
		return nil, ErrUnavailable
	}
	return u, nil
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

func newClient() *http.Client {
	dialer := &net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		MaxIdleConns: 16, MaxIdleConnsPerHost: 4, MaxConnsPerHost: 4,
		IdleConnTimeout: 60 * time.Second, TLSHandshakeTimeout: 3 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second, MaxResponseHeaderBytes: 32 << 10,
		ForceAttemptHTTP2: true,
		// Resolve once and dial the checked IP; TLS still verifies the original name.
		// Environment proxies are not used to bypass this destination policy.
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, ErrNetwork
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(ips) == 0 {
				return nil, ErrNetwork
			}
			for _, ip := range ips {
				if !publicIP(ip) {
					return nil, ErrNetwork
				}
			}
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
			}
			return nil, ErrNetwork
		},
	}
	return &http.Client{Transport: transport, Timeout: operationTimeout, CheckRedirect: redirectPolicy}
}

func redirectPolicy(r *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return ErrNetwork
	}
	if _, err := publicURL(r.URL.String()); err != nil {
		return ErrNetwork
	}
	return nil
}

type response struct {
	body    []byte
	url     *url.URL
	cookies bool
}

func (p *Provider) get(ctx context.Context, raw, accept string, limit int64) (response, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return response{}, ErrNetwork
	}
	r.Header.Set("Accept", accept)
	r.Header.Set("User-Agent", "ArixTV/experimental")
	// This request-local client shares the existing transport. Its redirect
	// closure records only cookie presence, including intermediate responses;
	// cookie values are never retained or replayed.
	client := *p.client
	redirectCookies := false
	baseRedirect := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if next.Response != nil && len(next.Response.Header.Values("Set-Cookie")) > 0 {
			redirectCookies = true
		}
		if baseRedirect != nil {
			return baseRedirect(next, via)
		}
		return redirectPolicy(next, via)
	}
	res, err := client.Do(r)
	if err != nil {
		if ctx.Err() != nil {
			return response{}, ctx.Err()
		}
		return response{}, ErrNetwork
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return response{}, ErrStatus
	}
	if res.ContentLength > limit {
		return response{}, ErrResponse
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		if ctx.Err() != nil {
			return response{}, ctx.Err()
		}
		return response{}, ErrNetwork
	}
	if int64(len(b)) > limit {
		return response{}, ErrResponse
	}
	return response{body: b, url: res.Request.URL, cookies: redirectCookies || len(res.Header.Values("Set-Cookie")) > 0}, nil
}
