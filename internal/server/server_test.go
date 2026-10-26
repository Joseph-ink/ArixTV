package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/auth"
	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/config"
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/store"
)

type mockProvider struct {
	calls   atomic.Int32
	resolve func(context.Context, channel.Channel) (channel.Stream, error)
}

func (*mockProvider) Discover(context.Context) ([]channel.Channel, error) {
	return nil, errors.New("not called in Phase 2")
}
func (p *mockProvider) Resolve(ctx context.Context, ch channel.Channel) (channel.Stream, error) {
	p.calls.Add(1)
	return p.resolve(ctx, ch)
}

type fixture struct {
	app             *App
	cfg             config.Config
	tokens          *auth.Store
	snapshots       *store.Store
	token, internal string
	runtime         *mockProvider
	logs            *bytes.Buffer
	candidate       store.Candidate
}

var testTime = time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

func setup(t *testing.T, edit func(*config.Config)) *fixture {
	t.Helper()
	c, err := config.Parse(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	c.Storage.DataDir = filepath.Join(root, "data")
	c.Storage.StateDir = filepath.Join(root, "state")
	c.Providers.Enabled = []string{"mock"}
	c.Remote.Enabled = true
	c.Remote.PublicOrigin = "https://tv.example.test"
	c.LAN.Enabled = true
	c.LAN.PublicOrigin = "http://192.168.50.10:8787"
	c.LAN.ListenAddresses = []string{"192.168.50.10"}
	c.LAN.AllowNetworks = []string{"192.168.50.0/24", "fd00::/64"}
	c.HTTP.RateBurst = 100
	c.HTTP.ShutdownTimeout = "1s"
	if edit != nil {
		edit(&c)
	}
	tokens := auth.NewStore(c.Storage.StateDir)
	token, _, err := tokens.Create("test device")
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.New(c.Storage.DataDir, c.Storage.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	candidate := store.Candidate{ProviderID: "mock", AttemptedAt: testTime, CompletedAt: testTime.Add(time.Second), Complete: true}
	for _, tc := range []struct {
		id   string
		drm  channel.DRM
		cap  channel.Capability
		kind channel.StreamType
	}{
		{"direct", channel.Clear, channel.ClearDirect, channel.HLS},
		{"aes", channel.HLSStandardEncrypted, channel.ClearDirect, channel.HLS},
		{"relay", channel.Clear, channel.ClearRelayOnly, channel.HLS},
		{"drm", channel.Widevine, channel.DRMPlayback, channel.DASH},
		{"unsupported", channel.Clear, channel.Unsupported, channel.HLS},
		{"unknown", channel.UnknownDRM, channel.UnknownCapability, channel.UnknownStream},
		{"opaque/id", channel.Clear, channel.ClearDirect, channel.HLS},
	} {
		candidate.Channels = append(candidate.Channels, channel.Channel{ProviderID: "mock", ProviderChannelID: tc.id, Name: tc.id, Group: `News "quoted"`, StreamType: tc.kind, DRM: tc.drm, Capability: tc.cap, LastSeen: testTime})
	}
	if _, err := snapshots.Commit(candidate); err != nil {
		t.Fatal(err)
	}
	runtime := &mockProvider{resolve: func(_ context.Context, ch channel.Channel) (channel.Stream, error) {
		return channel.Stream{URL: "https://cdn.example.test/live?signature=PRIVATE_PLACEHOLDER", StreamType: ch.StreamType, DRM: ch.DRM, Capability: ch.Capability}, nil
	}}
	registry, err := provider.NewRegistry(provider.Entry{Metadata: provider.Metadata{ID: "mock", Name: "Test only", Status: provider.Experimental, Access: provider.AnonymousFree}, Runtime: runtime})
	if err != nil {
		t.Fatal(err)
	}
	internal := "irv_" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	logs := &bytes.Buffer{}
	app, err := New(c, Options{Registry: registry, LogOutput: logs, InternalAuth: InternalDigest(sha256.Sum256([]byte(internal))), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	app.now = func() time.Time { return testTime.Add(time.Hour) }
	return &fixture{app: app, cfg: c, tokens: tokens, snapshots: snapshots, token: token.Reveal(), internal: internal, runtime: runtime, logs: logs, candidate: candidate}
}

func request(f *fixture, kind ListenerKind, target, peer string, edit func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.RemoteAddr = peer
	if edit != nil {
		edit(r)
	}
	w := httptest.NewRecorder()
	f.app.Handler(kind).ServeHTTP(w, r)
	return w
}
func (f *fixture) public(suffix string) string {
	return f.cfg.Remote.DirectPath + "/s/" + f.token + "/" + suffix
}
func assertCode(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d want %d: %q", w.Code, status, w.Body.String())
	}
}

func TestPublicRoutesAndAuthentication(t *testing.T) {
	f := setup(t, nil)
	for _, suffix := range []string{"playlist.m3u", "play/mock/direct"} {
		assertCode(t, request(f, LAN, f.public(suffix), "192.168.50.20:5000", nil), map[bool]int{true: 302, false: 200}[strings.HasPrefix(suffix, "play/")])
	}
	for _, path := range []string{"/", "/direct/healthz", "/healthz/", "/direct/s//playlist.m3u", "/direct/s/bad/playlist.m3u", f.public("play/missing/direct"), f.public("play/mock/missing"), f.public("play/mock/drm"), f.public("play/mock/relay"), f.public("play/mock/unsupported"), f.public("play/mock/unknown")} {
		assertCode(t, request(f, LAN, path, "192.168.50.20:5000", nil), 404)
	}
	if err := f.tokens.Revoke("test device"); err != nil {
		t.Fatal(err)
	}
	assertCode(t, request(f, LAN, f.public("playlist.m3u"), "192.168.50.20:5000", nil), 404)
	assertCode(t, request(f, Loopback, f.public("play/mock/direct"), "127.0.0.1:5000", nil), 404)
	if strings.Contains(f.logs.String(), f.token) {
		t.Fatal("token leaked in logs")
	}
}

func TestStrictPathsAndMethods(t *testing.T) {
	f := setup(t, nil)
	base := f.public("play/mock/direct")
	for _, path := range []string{strings.Replace(base, "/s/", "//s/", 1), base + "/", base + "?x=y", strings.Replace(base, "direct", "%64irect", 1), strings.Replace(base, "/mock/", "/../", 1), strings.Replace(base, "/direct", "/direct/..", 1), strings.Replace(base, "/mock/", "/mock%2f/", 1), strings.Replace(base, "/mock/", "/mock%5c/", 1), strings.Replace(base, "/mock/", "/%FF/", 1), strings.Replace(base, "/mock/", "/%00/", 1), strings.Replace(base, "/mock/", "/%2e%2e/", 1), strings.Replace(base, "/mock/", "/mock\\/", 1)} {
		w := request(f, Loopback, path, "127.0.0.1:5000", nil)
		assertCode(t, w, 404)
		if w.Header().Get("Location") != "" {
			t.Fatal("path was redirected")
		}
	}
	for _, method := range []string{"POST", "PUT", "OPTIONS", "DELETE", "TRACE"} {
		assertCode(t, request(f, Loopback, base, "127.0.0.1:5000", func(r *http.Request) { r.Method = method }), 404)
	}
	assertCode(t, request(f, Loopback, base, "127.0.0.1:5000", func(r *http.Request) { r.ContentLength = 1 }), 404)
	assertCode(t, request(f, Loopback, base, "127.0.0.1:5000", func(r *http.Request) { r.TransferEncoding = []string{"chunked"} }), 404)
	for _, suffix := range []string{"playlist.m3u", "play/mock/direct"} {
		w := request(f, Loopback, f.public(suffix), "127.0.0.1:5000", func(r *http.Request) { r.Method = "HEAD" })
		if w.Code != 200 && w.Code != 302 {
			t.Fatal(w.Code)
		}
		if w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
}

func TestLANACLAndForwardedSpoofing(t *testing.T) {
	f := setup(t, nil)
	target := f.public("playlist.m3u")
	for _, tc := range []struct {
		peer   string
		status int
	}{{"192.168.50.20:5000", 200}, {"192.168.51.20:5000", 404}, {"203.0.113.20:5000", 404}, {"[fd00::20]:5000", 200}, {"[fd01::20]:5000", 404}, {"[::ffff:192.168.50.20]:5000", 200}, {"bad-peer", 404}} {
		w := request(f, LAN, target, tc.peer, func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", "192.168.50.20")
			r.Header.Set("X-Forwarded-Proto", "https")
		})
		assertCode(t, w, tc.status)
	}
	assertCode(t, request(f, LAN, strings.Replace(target, f.token, "invalid", 1), "192.168.50.20:5000", nil), 404)
	mapped := setup(t, func(c *config.Config) { c.LAN.AllowNetworks = []string{"::ffff:192.168.50.0/120"} })
	assertCode(t, request(mapped, LAN, mapped.public("playlist.m3u"), "192.168.50.20:5000", nil), 200)
	assertCode(t, request(mapped, LAN, mapped.public("playlist.m3u"), "192.168.51.20:5000", nil), 404)
}

func TestTrustedProxyParsing(t *testing.T) {
	f := setup(t, nil)
	target := f.public("playlist.m3u")
	for _, tc := range []struct {
		xff, proto string
		status     int
	}{{"203.0.113.1", "https", 200}, {"2001:db8::1", "https", 200}, {"203.0.113.1, 203.0.113.2", "https", 404}, {"bad", "https", 404}, {" 203.0.113.1", "https", 404}, {"203.0.113.1", "http", 404}, {"203.0.113.1", "", 404}, {"0.0.0.0", "https", 404}} {
		w := request(f, Loopback, target, "127.0.0.1:5000", func(r *http.Request) {
			r.Header.Set("X-Forwarded-For", tc.xff)
			r.Header.Set("X-Forwarded-Proto", tc.proto)
		})
		assertCode(t, w, tc.status)
	}
	assertCode(t, request(f, Loopback, target, "127.0.0.1:5000", func(r *http.Request) {
		r.Header.Add("X-Forwarded-For", "203.0.113.1")
		r.Header.Add("X-Forwarded-For", "203.0.113.2")
		r.Header.Set("X-Forwarded-Proto", "https")
	}), 404)
	r := httptest.NewRequest("GET", target, nil)
	r.RemoteAddr = "[::1]:5000"
	r.Header.Set("X-Forwarded-For", "203.0.113.5")
	r.Header.Set("X-Forwarded-Proto", "bad")
	ip, ok := f.app.effectiveIP(Loopback, r)
	if !ok || ip.String() != "::1" {
		t.Fatal("untrusted loopback headers changed client identity")
	}
	assertCode(t, request(f, Loopback, target, "203.0.113.1:5000", func(r *http.Request) { r.Header.Set("X-Forwarded-For", "127.0.0.1") }), 404)
}

func TestPlaylistOriginsAndEligibility(t *testing.T) {
	f := setup(t, nil)
	for _, kind := range []ListenerKind{LAN, Loopback} {
		peer := "127.0.0.1:5000"
		origin := f.cfg.Remote.PublicOrigin
		if kind == LAN {
			peer = "192.168.50.20:5000"
			origin = f.cfg.LAN.PublicOrigin
		}
		w := request(f, kind, f.public("playlist.m3u"), peer, func(r *http.Request) {
			r.Host = "attacker.example"
			r.Header.Set("X-Forwarded-Host", "attacker.example")
		})
		assertCode(t, w, 200)
		body := w.Body.String()
		if strings.Count(body, "#EXTINF:") != 3 || !strings.HasPrefix(body, "#EXTM3U\n") {
			t.Fatal("playlist entries", body)
		}
		if !strings.Contains(body, origin+f.public("play/mock/aes")) || !strings.Contains(body, origin+f.public("play/mock/"+channelPath("opaque/id"))) {
			t.Fatal("advertised URL incorrect")
		}
		for _, forbidden := range []string{"attacker.example", "/relay", "/drm", "/unsupported", "/unknown", "xmltv", "x-tvg-url", "tvg-id", "cdn.example", "PRIVATE_PLACEHOLDER"} {
			if strings.Contains(body, forbidden) {
				t.Fatalf("unexpected playlist value %s", forbidden)
			}
		}
		if !strings.HasPrefix(w.Header().Get("Content-Type"), "audio/x-mpegurl") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("playlist headers")
		}
		second := request(f, kind, f.public("playlist.m3u"), peer, nil)
		if body != second.Body.String() {
			t.Fatal("nondeterministic output")
		}
		assertCode(t, request(f, kind, f.public("play/mock/"+channelPath("opaque/id")), peer, nil), 302)
	}
}

func TestResolverResultsAndSanitization(t *testing.T) {
	f := setup(t, nil)
	target := f.public("play/mock/direct")
	w := request(f, Loopback, target, "127.0.0.1:5000", nil)
	assertCode(t, w, 302)
	if w.Body.Len() != 0 || w.Header().Get("Location") != "https://cdn.example.test/live?signature=PRIVATE_PLACEHOLDER" {
		t.Fatal("redirect/body contract")
	}
	for name, resolve := range map[string]func(context.Context, channel.Channel) (channel.Stream, error){
		"error": func(context.Context, channel.Channel) (channel.Stream, error) {
			return channel.Stream{}, errors.New("PRIVATE_PLACEHOLDER upstream body cookie")
		},
		"panic": func(context.Context, channel.Channel) (channel.Stream, error) { panic("PRIVATE_PLACEHOLDER") },
		"relay result": func(context.Context, channel.Channel) (channel.Stream, error) {
			return channel.Stream{URL: "https://example.test", StreamType: channel.HLS, DRM: channel.Clear, Capability: channel.ClearRelayOnly}, nil
		},
		"protected": func(context.Context, channel.Channel) (channel.Stream, error) {
			return channel.Stream{URL: "https://example.test", StreamType: channel.DASH, DRM: channel.Widevine, Capability: channel.DRMPlayback}, nil
		},
		"expired": func(context.Context, channel.Channel) (channel.Stream, error) {
			return channel.Stream{URL: "https://example.test", StreamType: channel.HLS, DRM: channel.Clear, Capability: channel.ClearDirect, ExpiresAt: &testTime}, nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			f.runtime.resolve = resolve
			w := request(f, Loopback, target, "127.0.0.1:5000", nil)
			assertCode(t, w, 503)
			if w.Header().Get("Retry-After") != "30" || w.Header().Get("Location") != "" || strings.Contains(w.Body.String(), "PRIVATE_PLACEHOLDER") {
				t.Fatal("resolver failure disclosure")
			}
		})
	}
	if strings.Contains(f.logs.String(), "PRIVATE_PLACEHOLDER") || strings.Contains(f.logs.String(), f.token) || strings.Contains(f.logs.String(), "cdn.example") {
		t.Fatal("resolver logs leaked secret")
	}
}

func TestResolutionIsFreshAndNeverPersisted(t *testing.T) {
	f := setup(t, nil)
	f.runtime.resolve = func(_ context.Context, ch channel.Channel) (channel.Stream, error) {
		return channel.Stream{URL: fmt.Sprintf("https://cdn.example.test/live?signature=PRIVATE_PLACEHOLDER&sequence=%d", f.runtime.calls.Load()), StreamType: ch.StreamType, DRM: ch.DRM, Capability: ch.Capability}, nil
	}
	first := request(f, Loopback, f.public("play/mock/direct"), "127.0.0.1:5000", nil)
	second := request(f, Loopback, f.public("play/mock/direct"), "127.0.0.1:5000", nil)
	if first.Header().Get("Location") == second.Header().Get("Location") || f.runtime.calls.Load() != 2 {
		t.Fatal("resolved URL was cached")
	}
	data, err := os.ReadFile(filepath.Join(f.cfg.Storage.DataDir, "providers", "mock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data)+f.logs.String()+first.Body.String()+second.Body.String(), "PRIVATE_PLACEHOLDER") {
		t.Fatal("resolved URL persisted or disclosed outside Location")
	}
}

func TestInternalAndHealthIsolation(t *testing.T) {
	f := setup(t, nil)
	internal := "/internal/" + f.internal + "/play/mock/relay"
	assertCode(t, request(f, Loopback, internal, "127.0.0.1:5000", nil), 302)
	for _, path := range []string{internal, "/healthz"} {
		assertCode(t, request(f, LAN, path, "192.168.50.20:5000", nil), 404)
		assertCode(t, request(f, Loopback, path, "203.0.113.1:5000", nil), 404)
		assertCode(t, request(f, Loopback, path, "127.0.0.1:5000", func(r *http.Request) { r.Header.Set("X-Forwarded-For", "203.0.113.1") }), 404)
	}
	assertCode(t, request(f, Loopback, strings.Replace(internal, f.internal, f.token, 1), "127.0.0.1:5000", nil), 404)
	wrong := "irv_" + base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	assertCode(t, request(f, Loopback, strings.Replace(internal, f.internal, wrong, 1), "127.0.0.1:5000", nil), 404)
	f.app.internal = nil
	assertCode(t, request(f, Loopback, internal, "127.0.0.1:5000", nil), 404)
	w := request(f, Loopback, "/healthz", "[::1]:5000", nil)
	assertCode(t, w, 200)
	var health map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	if health["channel_count"] != float64(7) || health["snapshot_count"] != float64(1) {
		t.Fatal("health counts")
	}
	for _, secret := range []string{f.token, f.internal, f.cfg.Storage.StateDir, f.cfg.Storage.DataDir, "cdn.example", "PRIVATE_PLACEHOLDER"} {
		if strings.Contains(w.Body.String()+f.logs.String(), secret) {
			t.Fatal("health/log secret disclosure")
		}
	}
}

func TestStartupCorruptionAndEmptyCatalog(t *testing.T) {
	f := setup(t, nil)
	for _, path := range []string{filepath.Join(f.cfg.Storage.StateDir, "tokens.json"), filepath.Join(f.cfg.Storage.DataDir, "providers", "mock.json")} {
		old, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		bad := []byte(`{"schema_version":999}`)
		os.WriteFile(path, bad, 0600)
		if _, err := New(f.cfg, Options{Registry: f.app.registry}); err == nil {
			t.Fatal("corrupt startup state accepted")
		}
		persisted, _ := os.ReadFile(path)
		if !bytes.Equal(persisted, bad) {
			t.Fatal("corrupt state reset")
		}
		os.WriteFile(path, old, 0600)
	}
	f.cfg.Providers.Enabled = nil
	empty, _ := provider.NewRegistry()
	app, err := New(f.cfg, Options{Registry: empty})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/healthz", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	app.Handler(Loopback).ServeHTTP(w, r)
	assertCode(t, w, 200)
	if len(app.catalog.view.Load().ordered) != 0 {
		t.Fatal("disabled snapshots loaded")
	}
	missing := setup(t, nil)
	if err := os.Remove(filepath.Join(missing.cfg.Storage.DataDir, "providers", "mock.json")); err != nil {
		t.Fatal(err)
	}
	if app, err := New(missing.cfg, Options{Registry: missing.app.registry}); err != nil || !app.catalog.view.Load().degraded || len(app.catalog.view.Load().ordered) != 0 {
		t.Fatal("missing snapshot did not permit degraded startup", err)
	}
}
