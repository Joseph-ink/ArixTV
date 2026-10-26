package xumo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

func TestFreshResolveAndNoPlaybackPersistence(t *testing.T) {
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	c := ref(t, cs)
	s, err := f.p.Resolve(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if s.URL != "https://cdn.example.test/media.m3u8?signature=FAKE_MEDIA_SIGNATURE" || s.Validate(testNow) != nil || s.DRM != channel.Clear || s.Capability != channel.ClearDirect || s.ExpiresAt == nil || s.ExpiresAt.Unix() != 1893456600 {
		t.Fatal("invalid fresh resolution")
	}
	for path, count := range map[string]int{"/v2/proxy/channels/list/10006.json": 2, "/v2/channels/channel/99991158/broadcast.json": 1, "/v2/assets/asset/EXAMPLE_SIMULCAST_1.json": 1, "/master.m3u8": 1, "/media.m3u8": 1} {
		if f.count(path) != count {
			t.Fatal("wrong request budget/freshness")
		}
	}
	if strings.Contains(string(encoded(t, s)), "FAKE_MEDIA_SIGNATURE") {
		t.Fatal("ephemeral URL serialized")
	}
	if _, err := f.p.Resolve(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if f.count("/v2/assets/asset/EXAMPLE_SIMULCAST_1.json") != 2 {
		t.Fatal("stale asset cache used")
	}
}
func TestCurrentLookupAndProtectionRecheck(t *testing.T) {
	for _, edit := range []func(map[string]any){
		func(m map[string]any) { firstRow(m)["callsign"] = "EXAMPLE-DRM" },
		func(m map[string]any) { firstRow(m)["title"] = "Changed channel" },
		func(m map[string]any) { firstRow(m)["properties"].(map[string]any)["is_live"] = "false" },
	} {
		f := setup(t)
		cs, err := f.p.Discover(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		m := parse(t, "catalog.json")
		edit(m)
		f.set("/v2/proxy/channels/list/10006.json", fakeResponse{body: encoded(t, m)})
		if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err != ErrUnavailable {
			t.Fatal("unsafe current channel resolved")
		}
		if f.count("/v2/channels/channel/99991158/broadcast.json") != 0 {
			t.Fatal("protected channel queried for playback")
		}
	}
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.ProviderChannelID != referenceID {
			if _, err := f.p.Resolve(context.Background(), c); err != ErrUnavailable {
				t.Fatal("unqualified channel resolved")
			}
		}
	}
	c := ref(t, cs)
	c.ProviderChannelID = "777777"
	if _, err := f.p.Resolve(context.Background(), c); err != ErrUnavailable {
		t.Fatal("missing channel resolved")
	}
	c.ProviderID = "vizio"
	if _, err := f.p.Resolve(context.Background(), c); err != ErrUnavailable {
		t.Fatal("cross-provider identity resolved")
	}
}
func TestBroadcastAndAssetFailures(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"expired refresh":  func(m map[string]any) { m["refresh"].(map[string]any)["to"] = 1 },
		"future refresh":   func(m map[string]any) { m["refresh"].(map[string]any)["from"] = 4102444800 },
		"missing live":     func(m map[string]any) { delete(m["assets"].([]any)[0].(map[string]any), "live") },
		"wrong live type":  func(m map[string]any) { m["assets"].([]any)[0].(map[string]any)["live"] = "true" },
		"no current asset": func(m map[string]any) { m["assets"].([]any)[0].(map[string]any)["live"] = false },
		"multiple live assets": func(m map[string]any) {
			a := map[string]any{"id": "OTHER_ASSET", "live": true, "type": "Asset", "timestamps": map[string]int64{"start": 1893452400, "end": 1893459600}}
			m["assets"] = append(m["assets"].([]any), a)
		},
		"duplicate asset": func(m map[string]any) { m["assets"] = append(m["assets"].([]any), m["assets"].([]any)[0]) },
		"expired asset": func(m map[string]any) {
			m["assets"].([]any)[0].(map[string]any)["timestamps"].(map[string]any)["end"] = 100
		},
		"empty assets":     func(m map[string]any) { m["assets"] = []any{} },
		"unknown envelope": func(m map[string]any) { m["nextPage"] = "FAKE_CURSOR" },
		"unsafe asset ID":  func(m map[string]any) { m["assets"].([]any)[0].(map[string]any)["id"] = "../private" },
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			cs, err := f.p.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			m := parse(t, "broadcast.json")
			edit(m)
			f.set("/v2/channels/channel/99991158/broadcast.json", fakeResponse{body: encoded(t, m)})
			if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err == nil {
				t.Fatal("invalid current playback accepted")
			}
		})
	}
	for name, edit := range map[string]func(map[string]any){
		"wrong identity":      func(m map[string]any) { m["id"] = "OTHER_ASSET" },
		"VOD not live":        func(m map[string]any) { m["contentType"] = "EPISODIC" },
		"no provider":         func(m map[string]any) { m["providers"] = []any{} },
		"missing sources":     func(m map[string]any) { m["providers"].([]any)[0].(map[string]any)["sources"] = []any{} },
		"DRM source metadata": func(m map[string]any) { firstSource(m)["drm"] = true },
		"special headers": func(m map[string]any) {
			firstSource(m)["headers"] = map[string]string{"Authorization": "FAKE_CREDENTIAL"}
		},
		"unknown root":          func(m map[string]any) { m["newProtocol"] = true },
		"wrong format":          func(m map[string]any) { firstSource(m)["produces"] = "application/dash+xml" },
		"expired source":        func(m map[string]any) { firstSource(m)["uri"] = "https://cdn.example.test/media.m3u8?expires=1" },
		"invalid source expiry": func(m map[string]any) { firstSource(m)["uri"] = "https://cdn.example.test/media.m3u8?exp=invalid" },
		"unsafe source":         func(m map[string]any) { firstSource(m)["uri"] = "https://127.0.0.1/private" },
		"bad provider window":   func(m map[string]any) { m["providers"].([]any)[0].(map[string]any)["sunset"] = "invalid" },
		"past provider": func(m map[string]any) {
			m["providers"].([]any)[0].(map[string]any)["sunrise"] = "Mon, 1 Jan 2029 00:00:00 +0000"
			m["providers"].([]any)[0].(map[string]any)["sunset"] = "Mon, 1 Jan 2029 01:00:00 +0000"
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			cs, err := f.p.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			m := parse(t, "asset.json")
			edit(m)
			f.set("/v2/assets/asset/EXAMPLE_SIMULCAST_1.json", fakeResponse{body: encoded(t, m)})
			if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err == nil {
				t.Fatal("unsafe/invalid stream accepted")
			}
		})
	}
}
func TestHLSVariantsEncryptionAndIndirection(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
		drm        channel.DRM
	}{
		{"media", string(fixture(t, "media.m3u8")), true, channel.Clear},
		{"AES128", strings.Replace(string(fixture(t, "media.m3u8")), "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=AES-128,URI="key.bin"`, 1), true, channel.HLSStandardEncrypted},
		{"DRM", strings.Replace(string(fixture(t, "media.m3u8")), "#EXT-X-VERSION:3", `#EXT-X-KEY:METHOD=SAMPLE-AES,KEYFORMAT="com.apple.streamingkeydelivery",URI="skd://fake"`, 1), false, ""},
		{"invalid", "not HLS", false, ""},
		{"external audio", "#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"audio.m3u8\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\nmedia.m3u8\n", false, ""},
		{"deep master", "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nmaster.m3u8\n", false, ""},
		{"unsafe variant", "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\nhttps://127.0.0.1/private\n", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			cs, err := f.p.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			f.set("/master.m3u8", fakeResponse{body: []byte(tc.body), contentType: "application/vnd.apple.mpegurl"})
			s, err := f.p.Resolve(context.Background(), ref(t, cs))
			if tc.ok {
				if err != nil || s.DRM != tc.drm {
					t.Fatalf("resolution failed: %v", err)
				}
			} else if err == nil {
				t.Fatal("unsupported HLS returned")
			}
		})
	}
	for name, body := range map[string]any{"valid": "https://cdn.example.test/media.m3u8?signature=FAKE_POINTER_SIGNATURE", "invalid type": map[string]string{"uri": "https://cdn.example.test/media.m3u8"}, "unsafe": "https://10.0.0.1/private", "recursive": "https://cdn.example.test/master.m3u8"} {
		t.Run("pointer "+name, func(t *testing.T) {
			f := setup(t)
			cs, err := f.p.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(body)
			f.set("/master.m3u8", fakeResponse{body: b, contentType: "application/json"})
			s, err := f.p.Resolve(context.Background(), ref(t, cs))
			if name == "valid" {
				if err != nil || s.Validate(testNow) != nil || !strings.Contains(s.URL, "FAKE_POINTER_SIGNATURE") {
					t.Fatalf("pointer resolution: %v", err)
				}
			} else if err == nil {
				t.Fatal("unsafe pointer returned")
			}
		})
	}
}
func TestRelayOnlyDeliveryAndMacros(t *testing.T) {
	for _, path := range []string{"/v2/proxy/channels/list/10006.json", "/v2/channels/channel/99991158/broadcast.json", "/v2/assets/asset/EXAMPLE_SIMULCAST_1.json", "/master.m3u8", "/media.m3u8"} {
		f := setup(t)
		cs, err := f.p.Discover(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		f.mu.Lock()
		r := f.responses[path]
		r.cookie = "session=FAKE_COOKIE"
		f.mu.Unlock()
		f.set(path, r)
		s, err := f.p.Resolve(context.Background(), ref(t, cs))
		if err != nil || s.Capability != channel.ClearRelayOnly {
			t.Fatal("cookie continuity promoted to Direct")
		}
	}
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m := parse(t, "asset.json")
	firstSource(m)["uri"] = "https://cdn.example.test/master.m3u8?client_ip=192.0.2.1"
	f.set("/v2/assets/asset/EXAMPLE_SIMULCAST_1.json", fakeResponse{body: encoded(t, m)})
	s, err := f.p.Resolve(context.Background(), ref(t, cs))
	if err != nil || s.Capability != channel.ClearRelayOnly {
		t.Fatal("IP-bound stream promoted to Direct")
	}
	raw := "https://cdn.example.test/master.m3u8?sig=a%2Fb%2Bc%3D&device=%5BdeviceID%5D&publisher=[PPID]&lat=[LIMITED_AD_TRACKING]&app=[app_bundle]"
	expanded, err := f.p.expandSource(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(expanded, "sig=a%2Fb%2Bc%3D") || !strings.Contains(expanded, "device="+f.p.deviceID) || !strings.Contains(expanded, "publisher="+f.p.publisherID) || !strings.Contains(expanded, "lat=1") || !strings.Contains(expanded, "app=org.arixtv") {
		t.Fatal("query or identity macro damaged")
	}
	for _, raw := range []string{"https://cdn.example.test/[deviceID]/a.m3u8", "https://cdn.example.test/a.m3u8?x=[UNKNOWN]", "https://cdn.example.test/a.m3u8?x=[broken", "https://cdn.example.test/a.m3u8?x={UNKNOWN}"} {
		if _, err := f.p.expandSource(raw); err == nil {
			t.Fatal("unknown protocol identity accepted")
		}
	}
}

func TestFutureRefreshHintDoesNotDelayCurrentlyLiveAsset(t *testing.T) {
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b := parse(t, "broadcast.json")
	b["refresh"] = map[string]int64{"from": testNow.Add(55 * time.Minute).Unix(), "to": testNow.Add(63 * time.Minute).Unix()}
	f.set("/v2/channels/channel/99991158/broadcast.json", fakeResponse{body: encoded(t, b)})
	s, err := f.p.Resolve(context.Background(), ref(t, cs))
	if err != nil || s.Validate(testNow) != nil {
		t.Fatalf("future refresh hint rejected valid current asset: %v", err)
	}
	if s.ExpiresAt == nil || s.ExpiresAt.Unix() != testNow.Add(time.Hour).Unix() {
		t.Fatal("active asset upper bound not retained")
	}
}

func TestOpaqueAdQueryDoesNotInvalidateExpiryCheck(t *testing.T) {
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a := parse(t, "asset.json")
	firstSource(a)["uri"] = "https://cdn.example.test/master.m3u8?routing=FAKE_OPAQUE;CONTEXT&signature=a%2Fb%2Bc%3D"
	f.set("/v2/assets/asset/EXAMPLE_SIMULCAST_1.json", fakeResponse{body: encoded(t, a)})
	if _, err := f.p.Resolve(context.Background(), ref(t, cs)); err != nil {
		t.Fatalf("legitimate opaque query rejected: %v", err)
	}
	s := channel.Stream{}
	if err := urlExpiry(&s, "https://cdn.example.test/a.m3u8?routing=FAKE_OPAQUE;CONTEXT&expires=4102444800", testNow); err != nil || s.ExpiresAt == nil {
		t.Fatal("opaque query prevented recognized expiry validation")
	}
	for _, query := range []string{"exp=4102444800&exp=4102444801", "exp=4102444800&%65xp=4102444801", "expires=4102444800;invalid", "exp=%GG"} {
		if err := urlExpiry(&s, "https://cdn.example.test/a.m3u8?"+query, testNow); err == nil {
			t.Fatal("ambiguous/invalid actual expiry accepted")
		}
	}
}
