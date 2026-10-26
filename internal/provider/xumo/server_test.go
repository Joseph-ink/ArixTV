package xumo

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/auth"
	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/config"
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/provider/vizio"
	"github.com/Joseph-ink/ArixTV/internal/server"
	"github.com/Joseph-ink/ArixTV/internal/store"
)

func TestXumoSnapshotReloadAndServerCompatibility(t *testing.T) {
	f := setup(t)
	// Serving uses its actual clock. Give its resolver a fixed point captured at
	// setup and ample relative windows, avoiding a calendar-dependent expiry test.
	at := time.Now().UTC()
	f.p.now = func() time.Time { return at }
	b := parse(t, "broadcast.json")
	b["refresh"] = map[string]int64{"from": at.Add(-time.Minute).Unix(), "to": at.Add(time.Hour).Unix()}
	b["assets"].([]any)[0].(map[string]any)["timestamps"] = map[string]int64{"start": at.Add(-time.Hour).Unix(), "end": at.Add(time.Hour).Unix()}
	f.set("/v2/channels/channel/99991158/broadcast.json", fakeResponse{body: encoded(t, b)})
	a := parse(t, "asset.json")
	pr := a["providers"].([]any)[0].(map[string]any)
	pr["sunrise"] = at.Add(-time.Hour).Format("Mon, 2 Jan 2006 15:04:05 -0700")
	pr["sunset"] = at.Add(2 * time.Hour).Format("Mon, 2 Jan 2006 15:04:05 -0700")
	f.set("/v2/assets/asset/EXAMPLE_SIMULCAST_1.json", fakeResponse{body: encoded(t, a)})
	cfg, err := config.Parse(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cfg.Storage.DataDir = filepath.Join(root, "data")
	cfg.Storage.StateDir = filepath.Join(root, "state")
	cfg.Providers.Enabled = []string{"vizio", "xumo"}
	cfg.Remote.Enabled = true
	cfg.Remote.PublicOrigin = "https://tv.example.test"
	token, _, err := auth.NewStore(cfg.Storage.StateDir).Create("offline Xumo test")
	if err != nil {
		t.Fatal(err)
	}
	registry, err := provider.NewRegistry(provider.Entry{Metadata: vizio.Metadata(), Runtime: vizio.New()}, provider.Entry{Metadata: Metadata(), Runtime: f.p})
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	app, err := server.New(cfg, server.Options{Registry: registry, LogOutput: &logs})
	if err != nil {
		t.Fatal(err)
	}
	request := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:12345"
		r.Host = "untrusted.example.test"
		w := httptest.NewRecorder()
		app.Handler(server.Loopback).ServeHTTP(w, r)
		return w
	}
	prefix := "/direct/s/" + token.Reveal() + "/"
	if w := request(prefix + "playlist.m3u"); w.Code != 200 || strings.Contains(w.Body.String(), "/play/xumo/") {
		t.Fatal("missing snapshots should start empty")
	}
	channels, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshots, err := store.New(cfg.Storage.DataDir, cfg.Storage.StateDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapshots.Commit(store.Candidate{ProviderID: "xumo", AttemptedAt: at, CompletedAt: at.Add(time.Second), Complete: true, Channels: channels}); err != nil {
		t.Fatal(err)
	}
	// Existing Vizio identities coexist; its unchanged real-runtime resolver test
	// separately exercises Vizio HTTP. This test must not contact Vizio live.
	vc := channel.Channel{ProviderID: "vizio", ProviderChannelID: "existing-vizio", Name: "Existing Vizio fixture", LastSeen: at, StreamType: channel.HLS, DRM: channel.Clear, Capability: channel.ClearDirect}
	if _, err := snapshots.Commit(store.Candidate{ProviderID: "vizio", AttemptedAt: at, CompletedAt: at.Add(time.Second), Complete: true, Channels: []channel.Channel{vc}}); err != nil {
		t.Fatal(err)
	}
	if err := app.Reload(); err != nil {
		t.Fatal(err)
	}
	w := request(prefix + "playlist.m3u")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "https://tv.example.test"+prefix+"play/xumo/"+referenceID) || !strings.Contains(w.Body.String(), "/play/vizio/existing-vizio") || strings.Contains(w.Body.String(), "/play/xumo/90000002") || strings.Contains(w.Body.String(), "/play/xumo/90000003") || strings.Contains(w.Body.String(), "untrusted.example.test") {
		t.Fatal("incorrect reloaded multi-provider playlist")
	}
	w = request(prefix + "play/xumo/" + referenceID)
	if w.Code != http.StatusFound || w.Body.Len() != 0 || !strings.Contains(w.Header().Get("Location"), "FAKE_MEDIA_SIGNATURE") {
		t.Fatal("expected bodyless 302")
	}
	f.set("/v2/assets/asset/EXAMPLE_SIMULCAST_1.json", fakeResponse{status: 503, body: []byte("https://private.example.test/?token=FAKE_ERROR_BODY")})
	w = request(prefix + "play/xumo/" + referenceID)
	if w.Code != 503 || w.Header().Get("Retry-After") != "30" || strings.Contains(w.Body.String(), "FAKE_") {
		t.Fatal("unsafe resolver failure")
	}
	data, err := os.ReadFile(filepath.Join(cfg.Storage.DataDir, "providers", "xumo.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{token.Reveal(), f.p.deviceID, f.p.publisherID, "FAKE_", "UNUSED_PROGRAM", "EXAMPLE_SIMULCAST_1", "cdn.example.test"} {
		if bytes.Contains(data, []byte(secret)) || strings.Contains(logs.String(), secret) {
			t.Fatal("ephemeral URL/session/guide data persisted or logged")
		}
	}
}
