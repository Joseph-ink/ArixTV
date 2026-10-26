package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type HTTPConfig struct {
	ReadHeaderTimeout   string `toml:"read_header_timeout"`
	ReadTimeout         string `toml:"read_timeout"`
	WriteTimeout        string `toml:"write_timeout"`
	IdleTimeout         string `toml:"idle_timeout"`
	ResolveTimeout      string `toml:"resolve_timeout"`
	ShutdownTimeout     string `toml:"shutdown_timeout"`
	ReloadInterval      string `toml:"reload_interval"`
	MaxHeaderBytes      int    `toml:"max_header_bytes"`
	MaxConnections      int    `toml:"max_connections"`
	ResolverConcurrency int    `toml:"resolver_concurrency"`
	PlaylistPerMinute   int    `toml:"playlist_per_minute"`
	ResolvePerMinute    int    `toml:"resolve_per_minute"`
	RateBurst           int    `toml:"rate_burst"`
	RateMaxClients      int    `toml:"rate_max_clients"`
	RateClientTTL       string `toml:"rate_client_ttl"`
}

func (c Config) validateHTTP() error {
	for _, value := range []string{c.HTTP.ReadHeaderTimeout, c.HTTP.ReadTimeout, c.HTTP.WriteTimeout, c.HTTP.IdleTimeout, c.HTTP.ResolveTimeout, c.HTTP.ShutdownTimeout, c.HTTP.ReloadInterval, c.HTTP.RateClientTTL} {
		d, err := time.ParseDuration(value)
		if err != nil || d < time.Millisecond || d > 24*time.Hour {
			return errors.New("HTTP durations must be between 1ms and 24h")
		}
	}
	read, _ := time.ParseDuration(c.HTTP.ReadTimeout)
	header, _ := time.ParseDuration(c.HTTP.ReadHeaderTimeout)
	write, _ := time.ParseDuration(c.HTTP.WriteTimeout)
	resolve, _ := time.ParseDuration(c.HTTP.ResolveTimeout)
	if read < header || write <= resolve {
		return errors.New("HTTP read timeout must cover headers and write timeout must exceed resolver timeout")
	}
	ttl, _ := time.ParseDuration(c.HTTP.RateClientTTL)
	if c.HTTP.PlaylistPerMinute > 0 && c.HTTP.ResolvePerMinute > 0 && ttl < time.Duration(c.HTTP.RateBurst)*time.Minute/time.Duration(min(c.HTTP.PlaylistPerMinute, c.HTTP.ResolvePerMinute)) {
		return errors.New("rate client TTL must cover a full burst refill period")
	}
	if c.HTTP.MaxHeaderBytes < 1024 || c.HTTP.MaxHeaderBytes > 1<<20 || c.HTTP.MaxConnections < 1 || c.HTTP.MaxConnections > 4096 || c.HTTP.ResolverConcurrency < 1 || c.HTTP.ResolverConcurrency > 64 || c.HTTP.RateBurst < 1 || c.HTTP.RateBurst > 1000 || c.HTTP.RateMaxClients < 1 || c.HTTP.RateMaxClients > 65536 || c.HTTP.PlaylistPerMinute < 1 || c.HTTP.PlaylistPerMinute > 60000 || c.HTTP.ResolvePerMinute < 1 || c.HTTP.ResolvePerMinute > 60000 {
		return errors.New("HTTP resource limit outside supported bounds")
	}
	if len(c.LAN.ListenAddresses) > 16 || len(c.Providers.Enabled) > 64 || len(c.LAN.AllowNetworks) > 256 || len(c.Remote.TrustedProxies) > 256 {
		return errors.New("too many listeners or enabled providers")
	}
	if c.LAN.Enabled && c.LAN.PublicOrigin == "" {
		return errors.New("enabled LAN requires an explicit public_origin")
	}
	if c.LAN.PublicOrigin != "" && !validOrigin(c.LAN.PublicOrigin, "http") {
		return errors.New("lan.public_origin must be an HTTP origin without credentials, path, query or fragment")
	}
	if c.Remote.PublicOrigin != "" && !validOrigin(c.Remote.PublicOrigin, "https") {
		return errors.New("invalid remote HTTPS origin")
	}
	if len(c.Remote.DirectPath) > 128 {
		return errors.New("direct_path exceeds 128 bytes")
	}
	for _, part := range strings.Split(strings.TrimPrefix(c.Remote.DirectPath, "/"), "/") {
		if !safePathComponent(part) || part == "." || part == ".." {
			return errors.New("direct_path must use unreserved ASCII path components")
		}
	}
	first := strings.Split(strings.TrimPrefix(c.Remote.DirectPath, "/"), "/")[0]
	if first == "internal" || first == "healthz" {
		return errors.New("direct_path conflicts with reserved local routes")
	}
	return nil
}

func validOrigin(value, scheme string) bool {
	if len(value) > 2048 {
		return false
	}
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}
	return u.Scheme == scheme && u.Hostname() != "" && u.User == nil && u.Path == "" && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && !strings.Contains(value, "#") && !strings.HasSuffix(u.Host, ":")
}

func safePathComponent(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
			return false
		}
	}
	return true
}
