package xumo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/logsafe"
)

func TestDiscoveryNormalizationAndPersistence(t *testing.T) {
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 4 || f.count("/v2/proxy/channels/list/10006.json") != 1 {
		t.Fatal("incomplete discovery")
	}
	if f.count("/v2/channels/channel/99991158/broadcast.json") != 0 {
		t.Fatal("per-channel discovery probe")
	}
	c := ref(t, cs)
	if c.ProviderID != "xumo" || c.ProviderChannelID != referenceID || c.Name != referenceName || c.Group != "News" || c.Country != "US" || c.Language != "" || c.LogoURL != "" || !c.DirectEligible() {
		t.Fatalf("bad normalization: %+v", c)
	}
	if cs[0].Capability != channel.DRMPlayback || cs[1].Capability != channel.UnknownCapability || cs[2].Capability != channel.Unsupported {
		t.Fatal("unsafe classification")
	}
	for i, c := range cs {
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
		if i > 0 && cs[i-1].ProviderChannelID >= c.ProviderChannelID {
			t.Fatal("unstable ordering")
		}
	}
	b := encoded(t, cs)
	for _, s := range []string{"UNUSED_PROGRAM", "FAKE_SOURCE", "channelUrls", "providers", "signature", "publisher", "device"} {
		if bytes.Contains(b, []byte(s)) {
			t.Fatal("ephemeral or guide metadata serialized")
		}
	}
	again, err := f.p.Discover(context.Background())
	if err != nil || string(encoded(t, again)) != string(b) {
		t.Fatal("nondeterministic discovery")
	}
}
func TestDiscoveryIncompleteFailsClosed(t *testing.T) {
	tests := map[string]func(map[string]any){
		"missing ID":          func(m map[string]any) { delete(firstRow(m), "guid") },
		"missing name":        func(m map[string]any) { firstRow(m)["title"] = " " },
		"invalid ID":          func(m map[string]any) { firstRow(m)["guid"] = map[string]string{"value": "../unsafe"} },
		"missing callsign":    func(m map[string]any) { delete(firstRow(m), "callsign") },
		"duplicate IDs":       func(m map[string]any) { m["channel"].(map[string]any)["item"] = append(catalogRows(m), firstRow(m)) },
		"last row malformed":  func(m map[string]any) { catalogRows(m)[3].(map[string]any)["title"] = 42 },
		"empty":               func(m map[string]any) { m["channel"].(map[string]any)["item"] = []any{} },
		"missing set":         func(m map[string]any) { delete(m, "channel") },
		"null set":            func(m map[string]any) { m["channel"].(map[string]any)["item"] = nil },
		"unknown root":        func(m map[string]any) { m["newEnvelope"] = true },
		"next page":           func(m map[string]any) { m["next"] = "https://api.example.test/page2" },
		"continuation":        func(m map[string]any) { m["channel"].(map[string]any)["continuationToken"] = "FAKE_CURSOR" },
		"inconsistent count":  func(m map[string]any) { m["channel"].(map[string]any)["totalCount"] = 100 },
		"unsupported version": func(m map[string]any) { m["version"] = 3 },
		"invalid live type":   func(m map[string]any) { firstRow(m)["properties"].(map[string]any)["is_live"] = true },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			m := parse(t, "catalog.json")
			edit(m)
			f.set("/v2/proxy/channels/list/10006.json", fakeResponse{body: encoded(t, m)})
			cs, err := f.p.Discover(context.Background())
			if err == nil || cs != nil {
				t.Fatal("partial result accepted")
			}
		})
	}
	for _, body := range []string{`{`, `null`, `{"version":2,"channel":{"item":[]}} trailing`, `{"version":2,"version":2,"channel":{}}`, `{"version":2,"channel":{"item":[null]}}`} {
		f := setup(t)
		f.set("/v2/proxy/channels/list/10006.json", fakeResponse{body: []byte(body)})
		if cs, err := f.p.Discover(context.Background()); err == nil || cs != nil {
			t.Fatal("malformed catalog accepted")
		}
	}
}
func TestConservativeProfileAndFingerprint(t *testing.T) {
	f := setup(t)
	cs, err := f.p.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	original := ref(t, cs).Fingerprint
	for _, tc := range []struct {
		name    string
		edit    func(map[string]any)
		cap     channel.Capability
		changed bool
	}{
		{"description", func(r map[string]any) { r["description"] = "UNUSED_EPG_CHANGE" }, channel.ClearDirect, false},
		{"name change", func(r map[string]any) { r["title"] = "Changed identity" }, channel.UnknownCapability, true},
		{"callsign change", func(r map[string]any) { r["callsign"] = "OTHER-CMS" }, channel.UnknownCapability, true},
		{"new security field", func(r map[string]any) { r["drm"] = true }, channel.UnknownCapability, true},
		{"unknown property", func(r map[string]any) { r["properties"].(map[string]any)["unknown_protection"] = true }, channel.UnknownCapability, true},
		{"missing live", func(r map[string]any) { delete(r["properties"].(map[string]any), "is_live") }, channel.UnknownCapability, true},
		{"not simulcast", func(r map[string]any) { r["properties"].(map[string]any)["is_simulcast"] = "false" }, channel.UnknownCapability, true},
		{"DRM", func(r map[string]any) { r["callsign"] = "EXAMPLE-DRM" }, channel.DRMPlayback, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			m := parse(t, "catalog.json")
			tc.edit(firstRow(m))
			f.set("/v2/proxy/channels/list/10006.json", fakeResponse{body: encoded(t, m)})
			cs, err := f.p.Discover(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			c := ref(t, cs)
			if c.Capability != tc.cap || (c.Fingerprint != original) != tc.changed {
				t.Fatalf("incorrect qualification: %+v", c)
			}
		})
	}
	// Arbitrary embedded program fields are dropped, never interpreted or persisted.
	m := parse(t, "catalog.json")
	firstRow(m)["programs"] = []map[string]string{{"title": "UNUSED_GUIDE_TITLE"}}
	f.set("/v2/proxy/channels/list/10006.json", fakeResponse{body: encoded(t, m)})
	cs, err = f.p.Discover(context.Background())
	if err != nil || bytes.Contains(encoded(t, cs), []byte("UNUSED_GUIDE_TITLE")) {
		t.Fatal("guide content retained")
	}
	f.p.now = func() time.Time { return testNow.Add(time.Hour) }
	m = parse(t, "catalog.json")
	f.set("/v2/proxy/channels/list/10006.json", fakeResponse{body: encoded(t, m)})
	cs, err = f.p.Discover(context.Background())
	if err != nil || ref(t, cs).Fingerprint != original {
		t.Fatal("last-seen churned fingerprint")
	}
}
func TestAnonymousIdentityFormattingAndMetadata(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	q, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.deviceID) != 32 || len(p.publisherID) != 32 || p.deviceID == q.deviceID || p.publisherID == q.publisherID {
		t.Fatal("anonymous identities not independently random")
	}
	var out bytes.Buffer
	logsafe.New(&out).Info("provider initialized", "provider", p)
	b, _ := json.Marshal(p)
	formatted := fmt.Sprintf("%v %+v %#v", p, p, p) + out.String() + string(b)
	if strings.Contains(formatted, p.deviceID) || strings.Contains(formatted, p.publisherID) {
		t.Fatal("identity logged")
	}
	m := Metadata()
	if m.Validate() != nil || m.ID != "xumo" || string(m.Status) != "experimental" || string(m.Access) != "anonymous_free" || m.UnofficialMirror {
		t.Fatal("incorrect metadata")
	}
}
