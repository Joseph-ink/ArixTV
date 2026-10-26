package xumo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/logsafe"
)

func TestHTTPFailuresBoundsAndSecretSafety(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		declared int64
	}{
		{"status", 429, "https://private.example.test/?token=FAKE_UPSTREAM_SECRET", 0},
		{"malformed", 200, `{"channel":"FAKE_PARSE_SECRET"}`, 0},
		{"length cap", 200, "", maxCatalogBytes + 1},
		{"body cap", 200, strings.Repeat("x", maxCatalogBytes+1), 0},
		{"truncated", 200, "{", 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.declared > 0 {
					w.Header().Set("Content-Length", fmt.Sprint(tc.declared))
				}
				w.WriteHeader(tc.status)
				if tc.declared == 0 {
					w.(http.Flusher).Flush()
				}
				io.WriteString(w, tc.body)
			})
			cs, err := p.Discover(context.Background())
			if err == nil || cs != nil {
				t.Fatal("bad response accepted")
			}
			var logs bytes.Buffer
			logsafe.New(&logs).Error("provider failed", "error", err)
			if strings.Contains(err.Error()+logs.String(), "FAKE_") || strings.Contains(err.Error(), "http") {
				t.Fatal("error leaked body or URL")
			}
		})
	}
	p, _ := New()
	p.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("https://private.example.test/?token=FAKE_TRANSPORT_SECRET")
	})}
	if _, err := p.Discover(context.Background()); err != ErrNetwork {
		t.Fatal("transport error escaped")
	}
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v2/channels/channel/99991158/broadcast.json", "/v2/assets/asset/EXAMPLE_SIMULCAST_1.json"} {
		f := setup(t)
		f.set(path, fakeResponse{body: []byte("not JSON")})
		if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err == nil {
			t.Fatal("malformed playback JSON accepted")
		}
	}
	f.set("/master.m3u8", fakeResponse{body: bytes.Repeat([]byte{'x'}, maxManifestBytes+1)})
	if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err != ErrResponse {
		t.Fatal("manifest body cap missing")
	}
	f.set("/master.m3u8", fakeResponse{body: bytes.Repeat([]byte{' '}, maxPointerBytes+1), contentType: "application/json"})
	if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err != ErrProtocol {
		t.Fatal("indirection body cap missing")
	}
}
func TestCancellationDeadlineAndConcurrency(t *testing.T) {
	p, _ := New()
	started := make(chan struct{}, 2)
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		started <- struct{}{}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := p.Discover(ctx); done <- err }()
	}
	<-started
	<-started
	if _, err := p.Discover(context.Background()); err != ErrBusy {
		t.Fatal("unbounded provider work")
	}
	cancel()
	for i := 0; i < 2; i++ {
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatal("discovery did not cancel")
		}
	}
	short, stop := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer stop()
	if _, err := p.Discover(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("request deadline not enforced")
	}
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	began := make(chan struct{})
	f.p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(began)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	go func() { _, err := f.p.Resolve(ctx, ref(t, cs)); done <- err }()
	<-began
	cancel2()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("resolver did not cancel")
	}
	if _, err := p.Discover(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("already canceled request accepted")
	}
}
func TestRedirectsAndDestinationPolicy(t *testing.T) {
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	f.set("/master.m3u8", fakeResponse{status: 302, location: "https://cdn.example.test/media.m3u8", cookie: "session=FAKE_REDIRECT_COOKIE"})
	s, err := f.p.Resolve(context.Background(), ref(t, cs))
	if err != nil || s.Capability != channel.ClearRelayOnly {
		t.Fatal("redirect cookie not conservative")
	}
	for _, destination := range []string{"http://cdn.example.test/media.m3u8", "https://127.0.0.1/private", "https://10.0.0.1/private", "https://user:fake@cdn.example.test/media.m3u8"} {
		f.set("/master.m3u8", fakeResponse{status: 302, location: destination})
		if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err == nil {
			t.Fatal("unsafe redirect accepted")
		}
	}
	f.set("/master.m3u8", fakeResponse{status: 302, location: "https://cdn.example.test/master.m3u8"})
	before := f.count("/master.m3u8")
	if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err == nil {
		t.Fatal("redirect loop accepted")
	}
	if f.count("/master.m3u8")-before != 3 {
		t.Fatal("redirect bound incorrect")
	}
	for _, raw := range []string{"https://[::1]/a", "https://localhost/a", "https://127.0.0.1/a", "https://[::ffff:127.0.0.1]/a", "https://cdn.example.test:123/a", "https://cdn.example.test/a#frag", "https://user:fake@cdn.example.test/a"} {
		if _, err := publicURL(raw); err == nil {
			t.Fatal("unsafe destination accepted")
		}
	}
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::ffff:127.0.0.1", "fd00::1"} {
		if publicIP(netip.MustParseAddr(ip)) {
			t.Fatal("private dial allowed")
		}
	}
	var calls atomic.Int32
	f.p.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("not expected") })}
	if _, err := f.p.get(context.Background(), "https://127.0.0.1/private", "application/json", 10); err == nil || calls.Load() != 0 {
		t.Fatal("private URL reached transport")
	}
}
func TestAPIHeadersDoNotFollowCrossHostRedirect(t *testing.T) {
	f := setup(t)
	f.set("/v2/proxy/channels/list/10006.json", fakeResponse{status: 302, location: "https://cdn.example.test/redirected-catalog"})
	f.set("/redirected-catalog", fakeResponse{body: fixture(t, "catalog.json"), contentType: "application/json"})
	cs, err := f.p.Discover(context.Background())
	if err != nil || !ref(t, cs).DirectEligible() {
		t.Fatal("safe catalog redirect failed")
	}
}
func TestTLSVerificationRemainsEnabled(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted TLS request accepted") }))
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.StartTLS()
	defer s.Close()
	p, _ := New()
	transport := p.client.Transport.(*http.Transport)
	defer transport.CloseIdleConnections()
	if transport.Proxy != nil || transport.MaxConnsPerHost == 0 || transport.ResponseHeaderTimeout <= 0 || p.client.Timeout <= 0 {
		t.Fatal("unsafe HTTP defaults")
	}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, s.Listener.Addr().String())
	}
	if _, err := p.Discover(context.Background()); err != ErrNetwork {
		t.Fatal("untrusted certificate accepted")
	}
}
func TestProtocolBounds(t *testing.T) {
	if _, err := array[jsonPlaceholder]([]byte("["+strings.Repeat("{},", maxChannels)+"{}]"), maxChannels); err == nil {
		t.Fatal("allocation cap missing")
	}
	for _, raw := range []string{"null", "[]", `{"a":1,"a":2}`, `{"unknown":true}`} {
		if _, err := object([]byte(raw), "a"); err == nil {
			t.Fatal("malformed/unknown envelope accepted")
		}
	}
}

type jsonPlaceholder struct{}
