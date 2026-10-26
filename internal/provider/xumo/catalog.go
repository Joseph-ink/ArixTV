package xumo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

const maxCatalogBytes = 2 << 20
const maxChannels = 2000

// This is a deliberately narrow positive evidence profile, not a claim that
// non-DRM callsigns are clear. Other channels remain visible as unknown until
// independently validated. See docs/providers/XUMO.md for dated evidence.
const referenceID = "99991158"
const referenceCallsign = "XCBSNWS-CMS"
const referenceName = "CBS News 24/7"

type catalogRow struct {
	Guid struct {
		Value string `json:"value"`
	} `json:"guid"`
	Name     string `json:"title"`
	Callsign string `json:"callsign"`
	Genres   []struct {
		Value string `json:"value"`
	} `json:"genre"`
	Properties map[string]json.RawMessage `json:"properties"`
	Uncertain  bool                       `json:"-"`
}

func (r *catalogRow) UnmarshalJSON(raw []byte) error {
	fields, err := object(raw)
	if err != nil {
		return err
	}
	type plain catalogRow
	var decoded plain
	if json.Unmarshal(raw, &decoded) != nil {
		return ErrProtocol
	}
	*r = catalogRow(decoded)
	for key := range fields {
		switch key {
		case "guid", "title", "callsign", "genre", "properties", "description", "number":
		default:
			r.Uncertain = true
		}
	}
	return nil
}

func validID(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func (p *Provider) catalog(ctx context.Context) ([]channel.Channel, error) {
	res, err := p.get(ctx, apiOrigin+catalogPath, "application/json", maxCatalogBytes)
	if err != nil {
		return nil, err
	}
	top, err := object(res.body, "channel", "version")
	if err != nil {
		return nil, err
	}
	var version float64
	if json.Unmarshal(top["version"], &version) != nil || version != 2 {
		return nil, ErrProtocol
	}
	container, err := object(top["channel"], "item", "description", "link", "title")
	if err != nil {
		return nil, err
	}
	rows, err := array[catalogRow](container["item"], maxChannels)
	if err != nil || len(rows) == 0 {
		return nil, ErrProtocol
	}
	seen := make(map[string]bool, len(rows))
	result := make([]channel.Channel, 0, len(rows))
	now := p.now().UTC()
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !validID(row.Guid.Value) || seen[row.Guid.Value] || strings.TrimSpace(row.Callsign) == "" {
			return nil, ErrProtocol
		}
		seen[row.Guid.Value] = true
		c, err := normalize(row, now, res.cookies)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProviderChannelID < result[j].ProviderChannelID })
	return result, nil
}
func normalize(row catalogRow, now time.Time, cookies bool) (channel.Channel, error) {
	c := channel.Channel{ProviderID: "xumo", ProviderChannelID: row.Guid.Value, Name: strings.TrimSpace(row.Name), Country: "US", LastSeen: now, StreamType: channel.UnknownStream, DRM: channel.UnknownDRM, Capability: channel.UnknownCapability}
	if len(row.Genres) > 0 {
		c.Group = strings.TrimSpace(row.Genres[0].Value)
	}
	var live, simulcast string
	if value, ok := row.Properties["is_live"]; ok {
		if json.Unmarshal(value, &live) != nil || (live != "true" && live != "false") {
			return channel.Channel{}, ErrProtocol
		}
	}
	if value, ok := row.Properties["is_simulcast"]; ok {
		if json.Unmarshal(value, &simulcast) != nil || (simulcast != "true" && simulcast != "false") {
			return channel.Channel{}, ErrProtocol
		}
	}
	known := !row.Uncertain
	for key := range row.Properties {
		switch key {
		case "alt_brand_color", "amazon_tif_rating", "brand_color", "has_discontinuity", "has_vod", "hybrid_type", "is_amazon_tif_channel", "is_google_movie_channel", "is_live", "is_new", "is_opengmrss_movie_channel", "is_roku_movie_channel", "is_simulcast", "is_xumo_movie_channel", "simulcast_only":
		default:
			known = false
		}
	}
	switch {
	case strings.HasSuffix(row.Callsign, "-DRM") || strings.HasSuffix(row.Callsign, "DRM-CMS"):
		c.DRM = channel.OtherDRM
		c.Capability = channel.DRMPlayback
	case live == "false":
		c.Capability = channel.Unsupported
	case known && live == "true" && simulcast == "true" && row.Guid.Value == referenceID && row.Callsign == referenceCallsign && c.Name == referenceName:
		c.StreamType = channel.HLS
		c.DRM = channel.Clear
		c.Capability = channel.ClearDirect
		if cookies {
			c.Capability = channel.ClearRelayOnly
		}
	}
	if c.Validate() != nil {
		return channel.Channel{}, ErrProtocol
	}
	identity := c
	identity.LastSeen = time.Time{}
	b, _ := json.Marshal(struct {
		Channel                   channel.Channel
		Callsign, Live, Simulcast string
	}{identity, row.Callsign, live, simulcast})
	digest := sha256.Sum256(b)
	c.Fingerprint = hex.EncodeToString(digest[:])
	return c, nil
}
