package vizio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/logsafe"
)

var fixedTime = time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func catalogFixture(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if e := json.Unmarshal(fixture(t, "catalog.json"), &m); e != nil {
		t.Fatal(e)
	}
	return m
}
func first(m map[string]any) map[string]any { return m["channels"].([]any)[0].(map[string]any) }
func encode(t *testing.T, m any) []byte {
	t.Helper()
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// The real runtime uses a local TLS test server via an injected transport. The
// transport preserves original URLs in responses so relative HLS URLs exercise
// production parsing; certificates are trusted via httptest's client, never InsecureSkipVerify.
func localProvider(t *testing.T, h http.HandlerFunc) *Provider {
	t.Helper()
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	p := New()
	original := p.client
	target, _ := url.Parse(s.URL)
	transport := s.Client().Transport
	p.client = &http.Client{Timeout: operationTimeout, CheckRedirect: original.CheckRedirect, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		c := r.Clone(r.Context())
		u := *r.URL
		u.Scheme = target.Scheme
		u.Host = target.Host
		c.URL = &u
		c.Host = target.Host
		res, e := transport.RoundTrip(c)
		if res != nil {
			res.Request = r
		}
		return res, e
	})}
	p.now = func() time.Time { return fixedTime }
	t.Cleanup(p.client.CloseIdleConnections)
	return p
}
func contentProvider(t *testing.T, catalog []byte, manifest string) *Provider {
	return localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credentials sent")
		}
		if r.Header.Get("User-Agent") != "ArixTV/experimental" {
			t.Error("unexpected client identity")
		}
		switch r.URL.Path {
		case "/api/channels":
			if r.Header.Get("Accept") != "application/json" {
				t.Error("catalog accept")
			}
			w.Write(catalog)
		case "/live.m3u8":
			io.WriteString(w, manifest)
		case "/media.m3u8":
			w.Write(fixture(t, "media.m3u8"))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(500)
		}
	})
}
func TestDiscoverAndSerialization(t *testing.T) {
	var requests atomic.Int32
	p := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/api/channels" {
			t.Error("discovery probed other endpoint")
		}
		w.Write(fixture(t, "catalog.json"))
	})
	channels, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(channels) != 3 || requests.Load() != 1 {
		t.Fatal("incomplete discovery or extra probes")
	}
	c := channels[0]
	if c.ProviderID != "vizio" || c.ProviderChannelID != "news-1" || c.Name != "Example News" || c.Group != "News" || c.Country != "US" || c.Language != "" || c.LogoURL != "https://images.example.test/news.png" || !c.DirectEligible() {
		t.Fatalf("normalization: %+v", c)
	}
	if channels[1].Capability != channel.DRMPlayback || channels[2].Capability != channel.UnknownCapability {
		t.Fatal("unsafe protection classification")
	}
	for _, c := range channels {
		if e := c.Validate(); e != nil {
			t.Fatal(e)
		}
	}
	b := encode(t, channels)
	for _, s := range []string{"FAKE_SIGNATURE", "UNUSED_", "channelUrls", "cdn.example.test", "license.example.test"} {
		if bytes.Contains(b, []byte(s)) {
			t.Fatalf("persisted unwanted field %s", s)
		}
	}
	again, e := p.Discover(context.Background())
	if e != nil || again[0].Fingerprint != c.Fingerprint || again[0].ProviderChannelID != c.ProviderChannelID {
		t.Fatal("unstable discovery")
	}
}
func TestCatalogFailuresReturnNoPartialData(t *testing.T) {
	tests := map[string]func(map[string]any){
		"missing ID":          func(m map[string]any) { delete(first(m), "channelId") },
		"missing name":        func(m map[string]any) { first(m)["channelName"] = "  " },
		"missing urls":        func(m map[string]any) { delete(first(m), "channelUrls") },
		"duplicate":           func(m map[string]any) { m["channels"] = append(m["channels"].([]any), first(m)) },
		"last row invalid":    func(m map[string]any) { m["channels"].([]any)[2].(map[string]any)["channelName"] = "" },
		"empty":               func(m map[string]any) { m["channels"] = []any{} },
		"null":                func(m map[string]any) { m["channels"] = nil },
		"missing channels":    func(m map[string]any) { delete(m, "channels") },
		"wrong type":          func(m map[string]any) { first(m)["channelUrls"] = 42 },
		"next page":           func(m map[string]any) { m["nextPage"] = "https://api.example.test/next?token=FAKE" },
		"cursor":              func(m map[string]any) { m["cursor"] = "FAKE_CURSOR" },
		"has more":            func(m map[string]any) { m["hasMore"] = true },
		"total":               func(m map[string]any) { m["totalCount"] = 4 },
		"channelIds mismatch": func(m map[string]any) { m["channelIds"] = []string{"news-1"} },
		"invalid date":        func(m map[string]any) { first(m)["mobileStreamEndDate"] = "not a date" },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			m := catalogFixture(t)
			edit(m)
			p := contentProvider(t, encode(t, m), "")
			cs, e := p.Discover(context.Background())
			if e == nil || cs != nil {
				t.Fatal("accepted incomplete catalog")
			}
		})
	}
	for _, body := range []string{`{`, `null`, `{"channels":[]} true`, `{"channels":[null]}`} {
		p := contentProvider(t, []byte(body), "")
		if cs, e := p.Discover(context.Background()); e == nil || cs != nil {
			t.Fatal("accepted malformed catalog")
		}
	}
}
func TestDiscoveryClassificationAndFingerprint(t *testing.T) {
	base := catalogFixture(t)
	p := contentProvider(t, encode(t, base), "")
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	fingerprint := cs[0].Fingerprint
	for _, tc := range []struct {
		name    string
		edit    func(map[string]any)
		cap     channel.Capability
		changed bool
	}{
		{"query rotation", func(c map[string]any) {
			c["channelUrls"] = []string{"https://cdn.example.test/live.m3u8?signature=OTHER_FAKE"}
		}, channel.ClearDirect, false},
		{"guide change", func(c map[string]any) { c["airingsKey"] = "CHANGED_UNUSED" }, channel.ClearDirect, false},
		{"name change", func(c map[string]any) { c["channelName"] = "Different" }, channel.ClearDirect, true},
		{"path change", func(c map[string]any) { c["channelUrls"] = []string{"https://cdn.example.test/changed.m3u8"} }, channel.ClearDirect, true},
		{"missing license", func(c map[string]any) { delete(c, "licenseUrl") }, channel.UnknownCapability, true},
		{"gated", func(c map[string]any) { c["tokenUrl"] = "https://auth.example.test/private" }, channel.Unsupported, true},
		{"protected", func(c map[string]any) { c["licenseUrl"] = "https://license.example.test/" }, channel.DRMPlayback, true},
		{"inactive", func(c map[string]any) { c["channelState"] = map[string]int{"id": 3} }, channel.UnknownCapability, true},
		{"expired mobile", func(c map[string]any) { c["mobileStreamEndDate"] = "2020-01-01T00:00:00Z" }, channel.Unsupported, true},
		{"future mobile", func(c map[string]any) { c["mobileStreamStartDate"] = "2070-01-01T00:00:00Z" }, channel.Unsupported, true},
		{"unknown macro", func(c map[string]any) {
			c["channelUrls"] = []string{"https://cdn.example.test/live.m3u8?x=%7BNEW_MACRO%7D"}
		}, channel.UnknownCapability, true},
		{"non HLS", func(c map[string]any) { c["channelUrls"] = []string{"https://cdn.example.test/live.mpd"} }, channel.UnknownCapability, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := catalogFixture(t)
			tc.edit(first(m))
			p := contentProvider(t, encode(t, m), "")
			cs, e := p.Discover(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			if cs[0].Capability != tc.cap || (cs[0].Fingerprint != fingerprint) != tc.changed {
				t.Fatalf("classification/fingerprint mismatch: %+v", cs[0])
			}
		})
	}
}
func TestResolveFreshCatalogAndHLS(t *testing.T) {
	var catalogs atomic.Int32
	p := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/channels":
			catalogs.Add(1)
			w.Write(fixture(t, "catalog.json"))
		case "/live.m3u8":
			if r.URL.Query().Get("dnt") != "1" || r.URL.Query().Get("app") != "ArixTV" || r.URL.Query().Get("signature") != "FAKE_SIGNATURE" {
				t.Error("macro/signed query damage")
			}
			w.Write(fixture(t, "master.m3u8"))
		case "/media.m3u8":
			w.Write(fixture(t, "media.m3u8"))
		default:
			t.Error("requested a key, segment or extra variant")
			w.WriteHeader(500)
		}
	})
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.Resolve(context.Background(), cs[0])
	if e != nil {
		t.Fatal(e)
	}
	if catalogs.Load() != 2 || s.Validate(fixedTime) != nil || s.URL != "https://cdn.example.test/media.m3u8?expires=4102444800&signature=FAKE_VARIANT_SIGNATURE" || s.ExpiresAt == nil || s.Capability != channel.ClearDirect {
		t.Fatal("not a fresh valid resolution")
	}
	if b := encode(t, s); bytes.Contains(b, []byte("FAKE_")) {
		t.Fatal("stream URL serialized")
	}
}
func TestResolveFailuresAndEncryption(t *testing.T) {
	media := string(fixture(t, "media.m3u8"))
	for _, tc := range []struct {
		name, body string
		want       channel.DRM
		ok         bool
	}{
		{"clear", media, channel.Clear, true},
		{"AES128", strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=AES-128,URI="key.bin"`, 1), channel.HLSStandardEncrypted, true},
		{"widevine", strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=SAMPLE-AES,KEYFORMAT="urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed",URI="data:FAKE"`, 1), "", false},
		{"AES DRM format", strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=AES-128,KEYFORMAT="com.apple.streamingkeydelivery",URI="skd://fake"`, 1), "", false},
		{"unknown method", strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=UNKNOWN`, 1), "", false},
		{"bad manifest", "not HLS", "", false},
		{"empty HLS", "#EXTM3U\n", "", false},
		{"external audio", "#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"audio.m3u8\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\nvideo.m3u8\n", "", false},
		{"deep master", "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nlive.m3u8\n", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := contentProvider(t, fixture(t, "catalog.json"), tc.body)
			cs, e := p.Discover(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			s, e := p.Resolve(context.Background(), cs[0])
			if tc.ok {
				if e != nil || s.DRM != tc.want {
					t.Fatalf("unexpected result: %v", e)
				}
			} else if e == nil {
				t.Fatal("accepted unsupported manifest")
			}
		})
	}
}
func TestResolveRejectsCurrentProtectionAndExpiry(t *testing.T) {
	for _, change := range []func(map[string]any){
		func(c map[string]any) { c["licenseUrl"] = "https://license.example.test/" },
		func(c map[string]any) { c["channelUrls"] = []string{"https://cdn.example.test/live.m3u8?expires=1"} },
		func(c map[string]any) {
			c["channelUrls"] = []string{"https://cdn.example.test/live.m3u8?expires=invalid"}
		},
		func(c map[string]any) { c["channelUrls"] = []string{"javascript:FAKE"} },
	} {
		m := catalogFixture(t)
		p := contentProvider(t, encode(t, m), string(fixture(t, "media.m3u8")))
		cs, e := p.Discover(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		change(first(m))
		p2 := contentProvider(t, encode(t, m), string(fixture(t, "media.m3u8")))
		if _, e := p2.Resolve(context.Background(), cs[0]); e == nil {
			t.Fatal("accepted changed unsafe result")
		}
	}
	p := contentProvider(t, fixture(t, "catalog.json"), "")
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, ch := range []channel.Channel{cs[1], cs[2], {}, func() channel.Channel { c := cs[0]; c.ProviderID = "other"; return c }(), func() channel.Channel { c := cs[0]; c.ProviderChannelID = "absent"; return c }()} {
		if _, e := p.Resolve(context.Background(), ch); e == nil {
			t.Fatal("resolved unavailable identity")
		}
	}
}
func TestHTTPBoundsAndSafeErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		length bool
	}{
		{"non2xx", 429, "SECRET_UPSTREAM_BODY", false}, {"invalid JSON", 200, `{"channels": "SECRET_JSON"}`, false},
		{"oversized content length", 200, "", true}, {"oversized chunked", 200, strings.Repeat("x", maxCatalogBytes+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.length {
					w.Header().Set("Content-Length", fmt.Sprint(maxCatalogBytes+1))
				}
				w.WriteHeader(tc.status)
				if !tc.length {
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					io.WriteString(w, tc.body)
				}
			})
			cs, e := p.Discover(context.Background())
			if e == nil || cs != nil {
				t.Fatal("accepted bad response")
			}
			var out bytes.Buffer
			logsafe.New(&out).Error("provider operation failed", "error", e)
			for _, secret := range []string{"SECRET_", "https://", "signature="} {
				if strings.Contains(e.Error()+out.String(), secret) {
					t.Fatal("error leaked upstream data")
				}
			}
		})
	}
	p := New()
	p.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("https://private.example.test/?token=SECRET_TRANSPORT")
	})}
	if _, e := p.Discover(context.Background()); e != ErrNetwork {
		t.Fatal("raw network error escaped")
	}
	p = contentProvider(t, fixture(t, "catalog.json"), strings.Repeat("x", maxManifestBytes+1))
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Resolve(context.Background(), cs[0]); e != ErrResponse {
		t.Fatal("unbounded manifest")
	}
}
func TestCancellationAndCapacity(t *testing.T) {
	started := make(chan struct{}, 2)
	released := make(chan struct{})
	var once sync.Once
	p := New()
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-r.Context().Done()
		once.Do(func() { close(released) })
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, e := p.Discover(ctx); done <- e }()
	}
	<-started
	<-started
	if _, e := p.Discover(context.Background()); e != ErrBusy {
		t.Fatal("unbounded concurrent work")
	}
	cancel()
	<-released
	for i := 0; i < 2; i++ {
		if e := <-done; !errors.Is(e, context.Canceled) {
			t.Fatalf("cancellation: %v", e)
		}
	}
	expired, stop := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer stop()
	if _, e := p.Discover(expired); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("deadline ignored")
	}
	ch := channel.Channel{ProviderID: "vizio", ProviderChannelID: "test", Name: "Test", LastSeen: fixedTime, StreamType: channel.HLS, DRM: channel.Clear, Capability: channel.ClearDirect}
	if _, e := p.Resolve(ctx, ch); !errors.Is(e, context.Canceled) {
		t.Fatal("resolver cancellation ignored")
	}
}
func TestPublicDestinationAndMacroPolicy(t *testing.T) {
	for _, raw := range []string{"http://cdn.example.test/a.m3u8", "https://127.0.0.1/a", "https://[::1]/a", "https://10.0.0.1/a", "https://localhost/a", "https://user:password@cdn.example.test/a", "https://cdn.example.test:123/a", "https://cdn.example.test/a#fragment"} {
		if _, e := publicURL(raw); e == nil {
			t.Fatal("unsafe upstream destination")
		}
	}
	for _, raw := range []string{"https://cdn.example.test/{BAD}/a.m3u8", "https://cdn.example.test/a.m3u8?v=%7BNEW%7D", "https://cdn.example.test/a.m3u8?v=%7BBROKEN"} {
		if _, e := expand(raw); e == nil {
			t.Fatal("unknown/unsafe macro accepted")
		}
	}
	raw := "https://cdn.example.test/a.m3u8?sig=a%2Fb%2Bc%3D&device=%7BDEVICE_MODEL%7D&dnt={DNT}"
	expanded, e := expand(raw)
	if e != nil || expanded != "https://cdn.example.test/a.m3u8?sig=a%2Fb%2Bc%3D&device=ArixTV&dnt=1" {
		t.Fatal("URL damaged by expansion")
	}
	for _, ip := range []string{"127.0.0.1", "::ffff:127.0.0.1", "192.168.1.1", "169.254.169.254", "fd00::1"} {
		if publicIP(netip.MustParseAddr(ip)) {
			t.Fatal("private dial allowed")
		}
	}
	client := newClient()
	defer client.CloseIdleConnections()
	for _, raw := range []string{"http://cdn.example.test/", "https://127.0.0.1/"} {
		r, _ := http.NewRequest("GET", raw, nil)
		if client.CheckRedirect(r, []*http.Request{{}}) == nil {
			t.Fatal("unsafe redirect allowed")
		}
	}
	r, _ := http.NewRequest("GET", "https://cdn.example.test/", nil)
	if client.CheckRedirect(r, make([]*http.Request, 3)) == nil {
		t.Fatal("unbounded redirects")
	}
}
func TestCookieResponseRequiresRelay(t *testing.T) {
	p := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/channels" {
			w.Write(fixture(t, "catalog.json"))
			return
		}
		w.Header().Set("Set-Cookie", "session=FAKE_COOKIE; Secure")
		w.Write(fixture(t, "media.m3u8"))
	})
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.Resolve(context.Background(), cs[0])
	if e != nil || s.Capability != channel.ClearRelayOnly {
		t.Fatal("cookie-dependent response promoted to Direct")
	}
}
