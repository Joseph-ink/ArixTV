package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/config"
)

func TestBoundedRateLimiter(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.HTTP.RateBurst = 1; c.HTTP.RateMaxClients = 2 })
	assertCode(t, request(f, Loopback, f.public("playlist.m3u"), "127.0.0.1:5000", nil), 200)
	assertCode(t, request(f, Loopback, f.public("playlist.m3u"), "127.0.0.1:5000", nil), 429)
	assertCode(t, request(f, Loopback, strings.Replace(f.public("playlist.m3u"), f.token, "invalid", 1), "127.0.0.1:5000", nil), 404)
	l := newLimiter(f.cfg.HTTP)
	now := testTime
	first := netip.MustParseAddr("192.0.2.1")
	second := netip.MustParseAddr("192.0.2.2")
	third := netip.MustParseAddr("192.0.2.3")
	if !l.allow(first, resolveRate, now) || l.allow(first, resolveRate, now) {
		t.Fatal("burst not enforced")
	}
	if !l.allow(first, resolveRate, now.Add(time.Minute)) {
		t.Fatal("quota did not refill")
	}
	if !l.allow(second, preauth, now) || l.allow(third, preauth, now) || len(l.clients) != 2 {
		t.Fatal("client map exceeded cap")
	}
	if !l.allow(third, preauth, now.Add(duration(f.cfg.HTTP.RateClientTTL)+time.Minute)) {
		t.Fatal("idle eviction failed")
	}
	if len(l.clients) > 2 {
		t.Fatal("map growth")
	}
}

func TestEffectiveClientRateIdentity(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.HTTP.RateBurst = 1 })
	call := func(peer, xff string) *httptest.ResponseRecorder {
		return request(f, Loopback, f.public("playlist.m3u"), peer, func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", xff)
			r.Header.Set("X-Forwarded-Proto", "https")
		})
	}
	assertCode(t, call("127.0.0.1:5000", "203.0.113.1"), 200)
	assertCode(t, call("127.0.0.1:5000", "203.0.113.1"), 429)
	assertCode(t, call("127.0.0.1:5000", "203.0.113.2"), 200)
	assertCode(t, call("[::1]:5000", "203.0.113.3"), 200)
	assertCode(t, call("[::1]:5000", "203.0.113.4"), 429)
}

func await(t *testing.T, event <-chan struct{}) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(3 * time.Second):
		t.Fatal("expected event did not occur")
	}
}

func TestResolverConcurrencyAndCancellation(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.HTTP.ResolverConcurrency = 1 })
	started := make(chan struct{})
	cancelled := make(chan struct{})
	f.runtime.resolve = func(ctx context.Context, _ channel.Channel) (channel.Stream, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return channel.Stream{}, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest("GET", f.public("play/mock/direct"), nil).WithContext(ctx)
	r.RemoteAddr = "127.0.0.1:5000"
	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); f.app.Handler(Loopback).ServeHTTP(first, r) }()
	await(t, started)
	busy := request(f, Loopback, f.public("play/mock/direct"), "127.0.0.1:5000", nil)
	assertCode(t, busy, 503)
	if f.runtime.calls.Load() != 1 || busy.Header().Get("Retry-After") != "30" {
		t.Fatal("global resolver bound bypassed")
	}
	cancel()
	await(t, cancelled)
	await(t, done)
	assertCode(t, first, 503)
	if len(f.app.resolvers) != 0 {
		t.Fatal("resolver permit leaked")
	}
}

func TestResolverDeadline(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.HTTP.ResolveTimeout = "1ms" })
	var deadline error
	f.runtime.resolve = func(ctx context.Context, _ channel.Channel) (channel.Stream, error) {
		<-ctx.Done()
		deadline = ctx.Err()
		return channel.Stream{}, deadline
	}
	assertCode(t, request(f, Loopback, f.public("play/mock/direct"), "127.0.0.1:5000", nil), 503)
	if !errors.Is(deadline, context.DeadlineExceeded) {
		t.Fatal("resolution deadline missing")
	}
}

func TestSnapshotReloadAndLKG(t *testing.T) {
	f := setup(t, nil)
	if err := os.WriteFile(filepath.Join(f.cfg.Storage.DataDir, "providers", ".arixtv-partial"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Reload(); err != nil {
		t.Fatal("temporary file was treated as completed snapshot", err)
	}
	target := f.public("playlist.m3u")
	before := request(f, Loopback, target, "127.0.0.1:5000", nil).Body.String()
	next := f.candidate
	next.AttemptedAt = next.CompletedAt.Add(time.Minute)
	next.CompletedAt = next.AttemptedAt.Add(time.Second)
	newChannel := next.Channels[0]
	newChannel.ProviderChannelID = "new-channel"
	newChannel.Name = "New channel"
	next.Channels = append(append([]channel.Channel(nil), next.Channels...), newChannel)
	if _, err := f.snapshots.Commit(next); err != nil {
		t.Fatal(err)
	}
	if body := request(f, Loopback, target, "127.0.0.1:5000", nil).Body.String(); body != before {
		t.Fatal("request saw uninstalled view")
	}
	if err := f.app.Reload(); err != nil {
		t.Fatal(err)
	}
	after := request(f, Loopback, target, "127.0.0.1:5000", nil).Body.String()
	if !strings.Contains(after, "new-channel") || after == before {
		t.Fatal("completed snapshot not reloaded")
	}
	path := filepath.Join(f.cfg.Storage.DataDir, "providers", "mock.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Reload(); err == nil {
		t.Fatal("corrupt update accepted")
	}
	if body := request(f, Loopback, target, "127.0.0.1:5000", nil).Body.String(); body != after {
		t.Fatal("corrupt reload erased LKG")
	}
	health := request(f, Loopback, "/healthz", "127.0.0.1:5000", nil)
	if !strings.Contains(health.Body.String(), `"status":"degraded"`) {
		t.Fatal("reload failure not reported")
	}
	persisted, _ := os.ReadFile(path)
	if string(persisted) != `{"schema_version":999}` {
		t.Fatal("corrupt snapshot silently rewritten")
	}
}

func TestHTTPHardeningAndDisabledRemote(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.Remote.Enabled = false })
	assertCode(t, request(f, Loopback, f.public("playlist.m3u"), "127.0.0.1:5000", nil), 404)
	assertCode(t, request(f, LAN, f.public("playlist.m3u"), "192.168.50.20:5000", nil), 200)
	s := f.app.httpServer(context.Background(), Loopback)
	if s.ReadHeaderTimeout <= 0 || s.ReadTimeout <= 0 || s.WriteTimeout <= 0 || s.IdleTimeout <= 0 || s.MaxHeaderBytes != 16384 || s.ErrorLog == nil {
		t.Fatal("unbounded HTTP defaults")
	}
	s.ErrorLog.Print("PRIVATE_PLACEHOLDER arbitrary transport error")
	if strings.Contains(f.logs.String(), "PRIVATE_PLACEHOLDER") {
		t.Fatal("raw transport error leaked")
	}
}

func TestTokenCorruptionWhileServingFailsClosed(t *testing.T) {
	f := setup(t, nil)
	path := filepath.Join(f.cfg.Storage.StateDir, "tokens.json")
	bad := []byte(`{"schema_version":999}`)
	os.WriteFile(path, bad, 0600)
	assertCode(t, request(f, Loopback, f.public("playlist.m3u"), "127.0.0.1:5000", nil), 404)
	data, _ := os.ReadFile(path)
	if string(data) != string(bad) {
		t.Fatal("corrupt token state reset")
	}
}

func TestHealthRefreshFailureAndStaleness(t *testing.T) {
	f := setup(t, nil)
	if err := f.snapshots.ObserveAttempt("mock", f.candidate.CompletedAt.Add(time.Minute), "discovery_failed"); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Reload(); err == nil {
		t.Fatal("failed observation was not reflected")
	}
	health := request(f, Loopback, "/healthz", "127.0.0.1:5000", nil).Body.String()
	if !strings.Contains(health, `"failed":1`) || !strings.Contains(health, `"status":"degraded"`) {
		t.Fatal("provider failure counts missing")
	}
	assertCode(t, request(f, Loopback, f.public("play/mock/direct"), "127.0.0.1:5000", nil), 302)
	path := filepath.Join(f.cfg.Storage.StateDir, "refresh", "mock.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := f.app.Reload(); err != nil {
		t.Fatal(err)
	}
	f.app.now = func() time.Time { return testTime.Add(24 * time.Hour) }
	health = request(f, Loopback, "/healthz", "127.0.0.1:5000", nil).Body.String()
	if !strings.Contains(health, `"stale":1`) {
		t.Fatal("snapshot age not reported")
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":999}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(f.cfg, Options{Registry: f.app.registry}); err == nil {
		t.Fatal("corrupt observation accepted at startup")
	}
}
