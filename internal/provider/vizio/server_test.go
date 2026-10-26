package vizio

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
	"github.com/Joseph-ink/ArixTV/internal/config"
	"github.com/Joseph-ink/ArixTV/internal/provider"
	"github.com/Joseph-ink/ArixTV/internal/server"
	"github.com/Joseph-ink/ArixTV/internal/store"
)

func TestVizioServerCompatibility(t *testing.T) {
	p := contentProvider(t, fixture(t, "catalog.json"), string(fixture(t, "media.m3u8")))
	cs, e := p.Discover(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := config.Parse(strings.NewReader(""))
	if e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	cfg.Storage.DataDir = filepath.Join(root, "data")
	cfg.Storage.StateDir = filepath.Join(root, "state")
	cfg.Providers.Enabled = []string{"vizio"}
	cfg.Remote.Enabled = true
	cfg.Remote.PublicOrigin = "https://tv.example.test"
	snapshots, e := store.New(cfg.Storage.DataDir, cfg.Storage.StateDir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = snapshots.Commit(store.Candidate{ProviderID: "vizio", Complete: true, Channels: cs, AttemptedAt: fixedTime, CompletedAt: fixedTime.Add(time.Second)}); e != nil {
		t.Fatal(e)
	}
	token, _, e := auth.NewStore(cfg.Storage.StateDir).Create("offline test")
	if e != nil {
		t.Fatal(e)
	}
	registry, e := provider.NewRegistry(provider.Entry{Metadata: Metadata(), Runtime: p})
	if e != nil {
		t.Fatal(e)
	}
	var logs bytes.Buffer
	app, e := server.New(cfg, server.Options{Registry: registry, LogOutput: &logs, Version: "test"})
	if e != nil {
		t.Fatal(e)
	}
	request := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:12345"
		r.Host = "injected.example.test"
		w := httptest.NewRecorder()
		app.Handler(server.Loopback).ServeHTTP(w, r)
		return w
	}
	prefix := "/direct/s/" + token.Reveal() + "/"
	playlist := request(prefix + "playlist.m3u")
	if playlist.Code != http.StatusOK || !strings.Contains(playlist.Body.String(), "https://tv.example.test"+prefix+"play/vizio/news-1") || strings.Contains(playlist.Body.String(), "protected-2") || strings.Contains(playlist.Body.String(), "unknown-3") {
		t.Fatal("provider did not fit catalog/M3U serving")
	}
	playback := request(prefix + "play/vizio/news-1")
	if playback.Code != http.StatusFound || playback.Body.Len() != 0 || !strings.Contains(playback.Header().Get("Location"), "FAKE_SIGNATURE") {
		t.Fatal("provider did not produce bodyless 302")
	}
	if _, e := os.Stat(filepath.Join(cfg.Storage.DataDir, "playlist.m3u")); !os.IsNotExist(e) {
		t.Fatal("HTTP generated persistent playlist")
	}
	snapshot, e := os.ReadFile(filepath.Join(cfg.Storage.DataDir, "providers", "vizio.json"))
	if e != nil {
		t.Fatal(e)
	}
	for _, secret := range []string{token.Reveal(), "FAKE_SIGNATURE", "cdn.example.test", "UNUSED_GUIDE_KEY"} {
		if bytes.Contains(snapshot, []byte(secret)) || strings.Contains(logs.String(), secret) {
			t.Fatal("secret/upstream payload persisted or logged")
		}
	}
}
