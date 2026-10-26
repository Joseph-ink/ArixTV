package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

func reply(w http.ResponseWriter, r *http.Request, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(text)))
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		w.Write([]byte(text))
	}
}

func (a *App) handle(kind ListenerKind, w http.ResponseWriter, r *http.Request) {
	start := a.now()
	class := "unknown"
	status := 404
	code := "denied"
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	defer func() {
		if recover() != nil {
			status = 503
			code = "handler_panic"
			w.Header().Set("Retry-After", "30")
			reply(w, r, 503, "temporarily unavailable\n")
		}
		// Do not log request URI, raw errors, headers, IPs, channel IDs or Location.
		args := []any{"request_class", class, "listener", string(kind), "status", status, "code", code, "latency", a.now().Sub(start)}
		if status >= 500 {
			a.logger.Warn("HTTP request completed", args...)
		} else if status == 404 || status == 429 || class == "health" {
			a.logger.Debug("HTTP request completed", args...)
		} else {
			a.logger.Info("HTTP request completed", args...)
		}
	}()
	deny := func() { reply(w, r, 404, "not found\n") }
	if a.stopping.Load() {
		status = 503
		code = "stopping"
		w.Header().Set("Retry-After", "30")
		reply(w, r, 503, "temporarily unavailable\n")
		return
	}
	if (r.Method != http.MethodGet && r.Method != http.MethodHead) || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
			r.Close = true
			w.Header().Set("Connection", "close")
		}
		deny()
		return
	}
	route, ok := parseRoute(r, a.cfg.Remote.DirectPath)
	if !ok {
		deny()
		return
	}
	class = route.class
	ip, ok := a.effectiveIP(kind, r)
	if !ok {
		deny()
		return
	}
	if route.class == "health" || route.class == "internal" {
		peer, _ := peerIP(r)
		if kind != Loopback || !peer.IsLoopback() || forwarded(r) {
			deny()
			return
		}
	}
	if route.class == "health" {
		status = 200
		code = "health"
		a.health(w, r)
		return
	}
	if (route.class == "playlist" || route.class == "resolve") && kind == Loopback && !a.cfg.Remote.Enabled {
		deny()
		return
	}
	if !a.limiter.allow(ip, preauth, a.now()) {
		deny()
		return
	}
	if route.class == "internal" {
		if a.internal == nil || !secretSyntax(route.secret, "irv_") || !a.internal.VerifyInternal(route.secret) {
			deny()
			return
		}
	} else {
		if !secretSyntax(route.secret, "atv_") {
			deny()
			return
		}
		valid, err := a.tokens.Verify(route.secret)
		if err != nil {
			a.logger.Warn("authentication unavailable", "code", "token_state_invalid")
		}
		if err != nil || !valid {
			deny()
			return
		}
	}
	rateClass := resolveRate
	if route.class == "playlist" {
		rateClass = playlistRate
	}
	if !a.limiter.allow(ip, rateClass, a.now()) {
		status = 429
		code = "rate_limited"
		w.Header().Set("Retry-After", "30")
		reply(w, r, status, "too many requests\n")
		return
	}
	if route.class == "playlist" {
		origin := a.cfg.Remote.PublicOrigin
		if kind == LAN {
			origin = a.cfg.LAN.PublicOrigin
		}
		channels := a.catalog.view.Load().ordered
		counter := &byteCounter{}
		writePlaylist(counter, channels, origin, a.cfg.Remote.DirectPath, route.secret)
		w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(counter.n))
		status = 200
		code = "playlist"
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			if err := writePlaylist(w, channels, origin, a.cfg.Remote.DirectPath, route.secret); err != nil {
				code = "playlist_write_failed"
			}
		}
		return
	}
	if !channel.ValidProviderID(route.provider) {
		deny()
		return
	}
	id, ok := channelID(route.channel)
	if !ok {
		deny()
		return
	}
	ch, ok := a.catalog.view.Load().channels[channelKey{route.provider, id}]
	internal := route.class == "internal"
	if !ok || !eligible(ch, internal) {
		deny()
		return
	}
	_, runtime, ok := a.registry.Lookup(route.provider)
	if !ok || runtime == nil {
		deny()
		return
	}
	select {
	case a.resolvers <- struct{}{}:
		defer func() { <-a.resolvers }()
	default:
		status = 503
		code = "resolver_busy"
		w.Header().Set("Retry-After", "30")
		reply(w, r, status, "temporarily unavailable\n")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), duration(a.cfg.HTTP.ResolveTimeout))
	defer cancel()
	stream, err := runtime.Resolve(ctx, ch)
	if err != nil || ctx.Err() != nil || stream.Validate(a.now()) != nil || !streamEligible(stream, internal) || len(stream.URL) > 8192 {
		status = 503
		code = "resolve_failed"
		w.Header().Set("Retry-After", "30")
		reply(w, r, status, "temporarily unavailable\n")
		return
	}
	w.Header().Set("Location", stream.URL)
	w.Header().Set("Content-Length", "0")
	status = 302
	code = "resolved"
	w.WriteHeader(status)
}

func eligible(ch channel.Channel, internal bool) bool {
	return ch.Validate() == nil && (ch.Capability == channel.ClearDirect || internal && ch.Capability == channel.ClearRelayOnly)
}
func streamEligible(s channel.Stream, internal bool) bool {
	return s.Capability == channel.ClearDirect || internal && s.Capability == channel.ClearRelayOnly
}

type byteCounter struct{ n int }

func (c *byteCounter) Write(b []byte) (int, error)       { c.n += len(b); return len(b), nil }
func (c *byteCounter) WriteString(s string) (int, error) { c.n += len(s); return len(s), nil }

// Render one M3U line at a time; a large catalog never becomes an extra complete
// in-memory playlist per client. This is text output, never upstream media I/O.
func writePlaylist(w io.Writer, channels []channel.Channel, origin, prefix, token string) error {
	if _, err := io.WriteString(w, "#EXTM3U\n"); err != nil {
		return err
	}
	replacer := strings.NewReplacer("\"", "'", "\\", "_")
	for _, ch := range channels {
		if !ch.DirectEligible() {
			continue
		}
		group := replacer.Replace(ch.Group)
		line := "#EXTINF:-1"
		if group != "" {
			line += " group-title=\"" + group + "\""
		}
		if _, err := io.WriteString(w, line+","+ch.Name+"\n"); err != nil {
			return err
		}
		if _, err := io.WriteString(w, origin+prefix+"/s/"+token+"/play/"+ch.ProviderID+"/"+channelPath(ch.ProviderChannelID)+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	v := a.catalog.view.Load()
	state := "ready"
	if v.degraded {
		state = "degraded"
	}
	count := 0
	providers := append([]snapshotHealth{}, v.providers...)
	counts := map[string]int{"healthy": 0, "stale": 0, "failed": 0, "unknown": 0, "refreshing": 0}
	var last *snapshotHealth
	for i := range providers {
		p := &providers[i]
		if p.LastSuccess != nil && p.State == "healthy" {
			age := a.now().Sub(*p.LastSuccess)
			interval := duration(a.cfg.Refresh.Interval)
			if age > interval && age-interval > interval {
				p.State = "stale"
				state = "degraded"
			}
		}
		switch p.State {
		case "healthy", "stale", "refreshing":
			counts[p.State]++
		case "failed", "reload_failed", "metadata_invalid":
			counts["failed"]++
		default:
			counts["unknown"]++
		}
		if p.Generation > 0 {
			count++
		}
		if p.LastSuccess != nil && (last == nil || p.LastSuccess.After(*last.LastSuccess)) {
			last = p
		}
	}
	result := struct {
		Status             string           `json:"status"`
		Version            string           `json:"version,omitempty"`
		ChannelCount       int              `json:"channel_count"`
		DirectChannelCount int              `json:"direct_channel_count"`
		EnabledProviders   int              `json:"enabled_provider_count"`
		SnapshotCount      int              `json:"snapshot_count"`
		LastSuccess        *time.Time       `json:"last_success,omitempty"`
		Providers          []snapshotHealth `json:"providers"`
		Counts             map[string]int   `json:"provider_counts"`
	}{Status: state, Version: a.version, ChannelCount: len(v.channels), DirectChannelCount: v.directCount, EnabledProviders: len(a.cfg.Providers.Enabled), SnapshotCount: count, Providers: providers, Counts: counts}
	if last != nil {
		result.LastSuccess = last.LastSuccess
	}
	body, _ := json.Marshal(result)
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(200)
	if r.Method != http.MethodHead {
		w.Write(body)
	}
}
