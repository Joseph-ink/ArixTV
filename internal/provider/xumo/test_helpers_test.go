package xumo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

var testNow = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func parse(t *testing.T, name string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(fixture(t, name), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func encoded(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func catalogRows(m map[string]any) []any       { return m["channel"].(map[string]any)["item"].([]any) }
func firstRow(m map[string]any) map[string]any { return catalogRows(m)[0].(map[string]any) }
func firstSource(m map[string]any) map[string]any {
	return m["providers"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)
}
func ref(t *testing.T, cs []channel.Channel) channel.Channel {
	t.Helper()
	for _, c := range cs {
		if c.ProviderChannelID == referenceID {
			return c
		}
	}
	t.Fatal("reference missing")
	return channel.Channel{}
}
func newTestProvider(t *testing.T, h http.HandlerFunc) *Provider {
	t.Helper()
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	target, _ := url.Parse(s.URL)
	transport := s.Client().Transport
	redirect := p.client.CheckRedirect
	p.client = &http.Client{Timeout: operationTimeout, CheckRedirect: redirect, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Hostname() == "valencia-app-mds.xumo.com" {
			if r.Header.Get("Origin") != "https://play.xumo.com" || r.Header.Get("Referer") != "https://play.xumo.com/" {
				t.Error("missing public API context")
			}
		} else if r.Header.Get("Origin") != "" || r.Header.Get("Referer") != "" {
			t.Error("API context forwarded to playback")
		}
		copy := r.Clone(r.Context())
		u := *r.URL
		u.Scheme = target.Scheme
		u.Host = target.Host
		copy.URL = &u
		copy.Host = target.Host
		res, err := transport.RoundTrip(copy)
		if res != nil {
			res.Request = r
		}
		return res, err
	})}
	p.now = func() time.Time { return testNow }
	t.Cleanup(p.client.CloseIdleConnections)
	return p
}

type fakeResponse struct {
	body                          []byte
	status                        int
	contentType, cookie, location string
}
type fixtureServer struct {
	p         *Provider
	mu        sync.Mutex
	responses map[string]fakeResponse
	calls     map[string]int
}

func setup(t *testing.T) *fixtureServer {
	t.Helper()
	f := &fixtureServer{responses: map[string]fakeResponse{}, calls: map[string]int{}}
	for path, name := range map[string]string{"/v2/proxy/channels/list/10006.json": "catalog.json", "/v2/channels/channel/99991158/broadcast.json": "broadcast.json", "/v2/assets/asset/EXAMPLE_SIMULCAST_1.json": "asset.json", "/master.m3u8": "master.m3u8", "/media.m3u8": "media.m3u8"} {
		content := "application/json"
		if name == "master.m3u8" || name == "media.m3u8" {
			content = "application/vnd.apple.mpegurl"
		}
		f.responses[path] = fakeResponse{body: fixture(t, name), status: 200, contentType: content}
	}
	f.p = newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			t.Error("credential or cookie sent")
		}
		if r.Header.Get("User-Agent") != "ArixTV/experimental" {
			t.Error("unexpected user agent")
		}
		if r.URL.Path == "/v2/proxy/channels/list/10006.json" {
			if r.URL.Query().Get("sort") != "hybrid" || len(r.URL.Query()) != 1 {
				t.Error("geo/device override in catalog")
			}
		}
		if r.URL.Path == "/v2/assets/asset/EXAMPLE_SIMULCAST_1.json" {
			if r.URL.RawQuery != "f=providers" {
				t.Error("requested guide/program asset data")
			}
		}
		if r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "https://play.xumo.com" {
			t.Error("bad API context")
		}
		if r.URL.Path == "/master.m3u8" || r.URL.Path == "/media.m3u8" {
			if r.Header.Get("Origin") != "" || r.Header.Get("Referer") != "" {
				t.Error("API headers leaked to playback")
			}
		}
		f.mu.Lock()
		f.calls[r.URL.Path]++
		res, ok := f.responses[r.URL.Path]
		f.mu.Unlock()
		if !ok {
			t.Errorf("unexpected endpoint (no keys/media/guide allowed): %s", r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if res.contentType != "" {
			w.Header().Set("Content-Type", res.contentType)
		}
		if res.cookie != "" {
			w.Header().Set("Set-Cookie", res.cookie)
		}
		if res.location != "" {
			w.Header().Set("Location", res.location)
		}
		w.WriteHeader(res.status)
		w.Write(res.body)
	})
	return f
}
func (f *fixtureServer) set(path string, r fakeResponse) {
	if r.status == 0 {
		r.status = 200
	}
	f.mu.Lock()
	f.responses[path] = r
	f.mu.Unlock()
}
func (f *fixtureServer) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[path]
}
