package vizio

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

func TestManifestInspectorBoundaries(t *testing.T) {
	media := string(fixture(t, "media.m3u8"))
	for _, body := range []string{
		string([]byte{255}), "#EXTM3U\n\x00",
		"#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\n", // missing media URI
		"#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\n",       // missing variant
		strings.Replace(media, "#EXT-X-VERSION:3", "#EXT-X-DEFINE:NAME=\"x\",VALUE=\"v\"", 1),
		strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-SESSION-KEY:METHOD=SAMPLE-AES,URI="skd://fake"`, 1),
		strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=AES-128,URI="https://127.0.0.1/key"`, 1),
		strings.Replace(media, "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=AES-128,URI="key?ip=192.0.2.1"`, 1),
	} {
		if _, _, e := inspectManifest(body); e == nil {
			t.Fatal("unsafe/invalid HLS accepted")
		}
	}
	master := "#EXTM3U\n#EXT-X-MEDIA:TYPE=SUBTITLES,URI=\"captions.m3u8\",GROUP-ID=\"s\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\nmedia.m3u8\n"
	if _, v, e := inspectManifest(master); e != nil || v != "media.m3u8" {
		t.Fatal("optional subtitles should not prevent muxed variant selection")
	}
	for _, attrs := range []string{`METHOD=AES-128,URI="unterminated`, `METHOD=NONE,METHOD=AES-128`, `METHOD=`, `METHOD="NONE",`, `bad=VALUE`, `METHOD="NONE"TRAILING`} {
		if _, e := attributes(attrs); e == nil {
			t.Fatal("invalid HLS attributes accepted")
		}
	}
	if a, e := attributes(`TYPE=SUBTITLES,NAME="one,two",URI="sub.m3u8"`); e != nil || a["NAME"] != "one,two" {
		t.Fatal("quoted comma mishandled")
	}
}
func TestIntermediateCookieAndExplicitIPBinding(t *testing.T) {
	p := localProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/channels" {
			w.Write(fixture(t, "catalog.json"))
			return
		}
		if r.URL.Path == "/live.m3u8" {
			w.Header().Set("Set-Cookie", "session=FAKE_INTERMEDIATE_COOKIE")
			w.Header().Set("Location", "https://cdn.example.test/media.m3u8")
			w.WriteHeader(302)
			return
		}
		if r.Header.Get("Cookie") != "" {
			t.Error("intermediate cookie replayed")
		}
		w.Write(fixture(t, "media.m3u8"))
	})
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	s, e := p.Resolve(context.Background(), cs[0])
	if e != nil || s.Capability != channel.ClearRelayOnly {
		t.Fatal("intermediate cookie not classified conservatively")
	}
	m := catalogFixture(t)
	first(m)["channelUrls"] = []string{"https://cdn.example.test/live.m3u8?client_ip=192.0.2.1"}
	p = contentProvider(t, encode(t, m), string(fixture(t, "media.m3u8")))
	cs, e = p.Discover(context.Background())
	if e != nil || cs[0].Capability != channel.ClearRelayOnly {
		t.Fatal("explicit IP binding promoted to Direct")
	}
	s, e = p.Resolve(context.Background(), cs[0])
	if e != nil || s.Capability != channel.ClearRelayOnly {
		t.Fatal("relay-only channel could not be resolved through provider boundary")
	}
}
func TestResolveInflightCancellation(t *testing.T) {
	p := contentProvider(t, fixture(t, "catalog.json"), "")
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	started := make(chan struct{})
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := p.Resolve(ctx, cs[0]); done <- e }()
	<-started
	cancel()
	if e := <-done; !errors.Is(e, context.Canceled) {
		t.Fatal("inflight resolver did not cancel")
	}
}
func TestRequestDeadlineAndCatalogBounds(t *testing.T) {
	p := New()
	p.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, e := p.Discover(ctx); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal("inflight request deadline ignored")
	}
	for _, body := range []string{`{"channels":[],"channels":[]}`, `{"channels":[],"pagination":{"next":"page2"}}`, `{"channels":[],"newEnvelopeField":true}`} {
		if _, e := decodeEnvelope([]byte(body)); e == nil {
			t.Fatal("ambiguous or changed envelope accepted")
		}
	}
	raw := "[" + strings.Repeat("{},", maxChannels) + "{}]"
	if _, e := decodeRows([]byte(raw)); e == nil {
		t.Fatal("channel allocation cap not enforced")
	}
	s := channel.Stream{}
	if e := expiry(&s, "https://cdn.example.test/?exp=4102444800&expires=4102444790", fixedTime); e != nil || s.ExpiresAt.Unix() != 4102444790 {
		t.Fatal("expiry not conservative")
	}
}
