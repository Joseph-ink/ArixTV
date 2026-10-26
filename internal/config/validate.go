package config

import (
	"errors"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/provider"
)

func (c Config) Validate() error {
	if err := c.validateHTTP(); err != nil {
		return err
	}
	for _, dir := range []string{c.Storage.DataDir, c.Storage.StateDir} {
		if strings.TrimSpace(dir) == "" || strings.ContainsAny(dir, "\x00\r\n") {
			return errors.New("storage paths must be nonempty local paths")
		}
	}
	addr, err := netip.ParseAddr(c.Server.LoopbackAddress)
	if err != nil || !addr.IsLoopback() || addr.Zone() != "" {
		return errors.New("server.loopback_address must be a loopback IP address")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}
	if c.LAN.Enabled && (len(c.LAN.ListenAddresses) == 0 || len(c.LAN.AllowNetworks) == 0) {
		return errors.New("enabled LAN requires listen_addresses and allow_networks")
	}
	for _, value := range c.LAN.ListenAddresses {
		addr, err := netip.ParseAddr(value)
		if err != nil || addr.Zone() != "" || addr.IsLoopback() || !addr.IsGlobalUnicast() {
			return errors.New("lan.listen_addresses must contain explicit unicast IP addresses")
		}
	}
	if !validPrefixes(c.LAN.AllowNetworks) {
		return errors.New("lan.allow_networks must contain valid CIDRs")
	}
	if !validPrefixes(c.Remote.TrustedProxies) {
		return errors.New("remote.trusted_proxies must contain valid CIDRs")
	}
	if c.Remote.Enabled && (c.Remote.PublicOrigin == "" || len(c.Remote.TrustedProxies) == 0) {
		return errors.New("enabled remote requires public_origin and trusted_proxies")
	}
	if c.Remote.PublicOrigin != "" {
		u, err := url.Parse(c.Remote.PublicOrigin)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Opaque != "" ||
			u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" ||
			strings.Contains(c.Remote.PublicOrigin, "#") {
			return errors.New("remote.public_origin must be an HTTPS origin without credentials, path, query or fragment")
		}
		if u.Port() != "" {
			port, err := netip.ParseAddrPort("127.0.0.1:" + u.Port())
			if err != nil || port.Port() == 0 {
				return errors.New("remote.public_origin has an invalid port")
			}
		}
	}
	if !strings.HasPrefix(c.Remote.DirectPath, "/") || c.Remote.DirectPath == "/" ||
		path.Clean(c.Remote.DirectPath) != c.Remote.DirectPath ||
		strings.ContainsAny(c.Remote.DirectPath, "?#%\\ \t\r\n") {
		return errors.New("remote.direct_path must be a clean absolute path without a trailing slash")
	}
	duration, err := time.ParseDuration(c.Refresh.Interval)
	if err != nil || duration <= 0 {
		return errors.New("refresh.interval must be a positive Go duration")
	}
	if c.Refresh.GlobalConcurrency < 1 || c.Refresh.ProviderConcurrency < 1 {
		return errors.New("refresh concurrency values must be positive")
	}
	seen := make(map[string]bool)
	for _, id := range c.Providers.Enabled {
		if !validProviderID(id) || seen[id] {
			return errors.New("providers.enabled must contain unique lowercase provider identifiers")
		}
		seen[id] = true
	}
	return nil
}

func validPrefixes(values []string) bool {
	for _, value := range values {
		if _, err := netip.ParsePrefix(value); err != nil {
			return false
		}
	}
	return true
}

func validProviderID(id string) bool {
	return channel.ValidProviderID(id)
}

// ValidateProviders is for operational commands once runtimes exist. The
// config-validation CLI intentionally remains syntax-only for provider IDs.
func (c Config) ValidateProviders(registry *provider.Registry) error {
	if registry == nil {
		return errors.New("provider registry is required")
	}
	return registry.ValidateEnabled(c.Providers.Enabled)
}
