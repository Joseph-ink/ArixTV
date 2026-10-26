package hlsinspect

import (
	"net/url"
	"strings"
	"testing"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

func public(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Hostname() == "cdn.example.test" && u.User == nil
}

const media = "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXTINF:6,\nsegment.ts\n"

func TestPureInspectionAndVariantSelection(t *testing.T) {
	for _, tc := range []struct {
		body    string
		drm     channel.DRM
		variant string
	}{
		{media, channel.Clear, ""},
		{strings.Replace(media, "#EXT-X-TARGETDURATION", `#EXT-X-KEY:METHOD=AES-128,URI="key.bin"`+"\n#EXT-X-TARGETDURATION", 1), channel.HLSStandardEncrypted, ""},
		{"#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"a\"\n#EXT-X-MEDIA:TYPE=SUBTITLES,URI=\"captions.m3u8\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\none.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=2\ntwo.m3u8\n", channel.Clear, "one.m3u8"},
	} {
		drm, variant, err := Inspect(tc.body, public)
		if err != nil || drm != tc.drm || variant != tc.variant {
			t.Fatal("inspection mismatch")
		}
	}
}
func TestConservativeProtectionAndGrammar(t *testing.T) {
	for _, body := range []string{
		"", string([]byte{255}), "#EXTM3U\n\x00", "#EXTM3U\n", "#EXTM3U\n#EXTINF:6,\n", "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\n",
		strings.Replace(media, "#EXT-X-TARGETDURATION:6", "#EXT-X-TARGETDURATION:bad", 1),
		strings.Replace(media, "#EXTINF:6,", "#EXTINF:6,\n#EXTINF:6,", 1),
		"#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=1\n#EXT-X-STREAM-INF:BANDWIDTH=2\nfile.m3u8\n",
		strings.Replace(media, "segment.ts", "extra.ts\nsegment.ts", 1),
		strings.Replace(media, "#EXT-X-TARGETDURATION", `#EXT-X-DEFINE:NAME="x",VALUE="y"`+"\n#EXT-X-TARGETDURATION", 1),
		strings.Replace(media, "#EXT-X-TARGETDURATION", `#EXT-X-KEY:METHOD=SAMPLE-AES,URI="skd://fake"`+"\n#EXT-X-TARGETDURATION", 1),
		strings.Replace(media, "#EXT-X-TARGETDURATION", `#EXT-X-KEY:METHOD=AES-128,URI="https://127.0.0.1/key"`+"\n#EXT-X-TARGETDURATION", 1),
		strings.Replace(media, "#EXT-X-TARGETDURATION", `#EXT-X-SESSION-KEY:METHOD=AES-128,KEYFORMAT="com.apple.streamingkeydelivery",URI="skd://fake"`+"\n#EXT-X-TARGETDURATION", 1),
		"#EXTM3U\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"a.m3u8\"\n#EXT-X-STREAM-INF:BANDWIDTH=1\nv.m3u8\n",
	} {
		if _, _, err := Inspect(body, public); err == nil {
			t.Fatal("unsupported input accepted")
		}
	}
	if _, _, err := Inspect(strings.Replace(media, "#EXT-X-TARGETDURATION", `#EXT-X-KEY:METHOD=NONE`+"\n#EXT-X-TARGETDURATION", 1), public); err != nil {
		t.Fatal("clear key reset rejected")
	}
}
func TestAttributesAndURIHelpers(t *testing.T) {
	if attrs, err := Attributes(`TYPE=SUBTITLES,NAME="a,b",URI="sub.m3u8"`); err != nil || attrs["NAME"] != "a,b" {
		t.Fatal("quoted comma mishandled")
	}
	for _, s := range []string{`METHOD`, `=NONE`, `method=NONE`, `METHOD=NONE,METHOD=AES-128`, `URI="broken`, `URI="x"trailing`, `URI="x",`, `METHOD=`} {
		if _, err := Attributes(s); err == nil {
			t.Fatal("invalid attribute list accepted")
		}
	}
	for _, s := range []string{"", "skd://fake", "//private/key", "https://user:fake@cdn.example.test/key", "https://127.0.0.1/key", "key#frag", "key\\unsafe", "{KEY}"} {
		if ValidURI(s, public) {
			t.Fatal("unsafe reference accepted")
		}
	}
	for _, s := range []string{"key.bin", "https://cdn.example.test/key"} {
		if !ValidURI(s, public) {
			t.Fatal("safe reference rejected")
		}
	}
	if !NeedsRelay("https://cdn.example.test/master.m3u8?client_ip=192.0.2.1") || NeedsRelay("%invalid") || NeedsRelay("https://cdn.example.test/master.m3u8?ip=") {
		t.Fatal("IP binding heuristic changed")
	}
}
