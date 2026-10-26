package vizio

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
)

const maxCatalogBytes = 4 << 20
const maxChannels = 2000

// Only channel fields are decoded. Unknown upstream fields are ignored, not
// retained as arbitrary metadata (in particular, no guide/program model exists).
type upstreamChannel struct {
	ID           string   `json:"channelId"`
	Name         string   `json:"channelName"`
	Logo         string   `json:"channelIcon"`
	Portrait     string   `json:"portraitIcon"`
	Group        string   `json:"category"`
	URLs         []string `json:"channelUrls"`
	License      *string  `json:"licenseUrl"`
	LicenseToken *string  `json:"licenseToken"`
	Token        *string  `json:"tokenUrl"`
	Type         string   `json:"channelType"`
	State        *struct {
		ID int `json:"id"`
	} `json:"channelState"`
	Start string `json:"mobileStreamStartDate"`
	End   string `json:"mobileStreamEndDate"`
}

type catalogEntry struct {
	channel  channel.Channel
	playback string
}

func (p *Provider) catalog(ctx context.Context) ([]catalogEntry, error) {
	res, err := p.get(ctx, p.endpoint, "application/json", maxCatalogBytes)
	if err != nil {
		return nil, err
	}
	envelope, err := decodeEnvelope(res.body)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows(envelope["channels"])
	if err != nil {
		return nil, ErrCatalog
	}
	// Current API is one complete response. Refuse known continuation/count
	// indicators instead of interpreting a future paginated response as complete.
	for _, key := range []string{"next", "nextPage", "nextPageToken", "continuationToken", "cursor", "hasMore"} {
		v := string(bytes.TrimSpace(envelope[key]))
		if v != "" && v != "null" && v != `""` && v != "false" {
			return nil, ErrCatalog
		}
	}
	for _, key := range []string{"total", "totalCount"} {
		if v, ok := envelope[key]; ok {
			var n int
			if json.Unmarshal(v, &n) != nil || n != len(rows) {
				return nil, ErrCatalog
			}
		}
	}
	seen := make(map[string]bool, len(rows))
	entries := make([]catalogEntry, 0, len(rows))
	now := p.now().UTC()
	for _, row := range rows {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if seen[row.ID] || row.ID == "" || len(row.URLs) == 0 || row.URLs[0] == "" {
			return nil, ErrCatalog
		}
		seen[row.ID] = true
		entry, err := normalize(row, now)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if v, ok := envelope["channelIds"]; ok {
		var ids []string
		if json.Unmarshal(v, &ids) != nil {
			return nil, ErrCatalog
		}
		// The current envelope contains an empty UI selection list. A nonempty
		// list must agree with the returned complete set; never silently truncate.
		if len(ids) > 0 {
			if len(ids) != len(rows) {
				return nil, ErrCatalog
			}
			idSet := map[string]bool{}
			for _, id := range ids {
				if !seen[id] || idSet[id] {
					return nil, ErrCatalog
				}
				idSet[id] = true
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].channel.ProviderChannelID < entries[j].channel.ProviderChannelID
	})
	return entries, nil
}

func normalize(row upstreamChannel, now time.Time) (catalogEntry, error) {
	c := channel.Channel{ProviderID: "vizio", ProviderChannelID: row.ID, Name: strings.TrimSpace(row.Name), Group: strings.TrimSpace(row.Group), Country: "US", LastSeen: now,
		StreamType: channel.UnknownStream, DRM: channel.UnknownDRM, Capability: channel.UnknownCapability}
	for _, logo := range []string{row.Logo, row.Portrait} {
		if u, err := publicURL(logo); err == nil && u.RawQuery == "" && !u.ForceQuery {
			c.LogoURL = logo
			break
		}
	}
	playback, err := expand(row.URLs[0])
	if err == nil {
		u, _ := url.Parse(playback)
		if strings.HasSuffix(strings.ToLower(u.Path), ".m3u8") {
			c.StreamType = channel.HLS
		}
	}
	// Explicit nonempty protection data always wins. No license/token endpoint
	// is called. Missing protection fields are unknown, never presumed empty.
	if (row.License != nil && *row.License != "") || (row.LicenseToken != nil && *row.LicenseToken != "") {
		c.DRM = channel.OtherDRM
		c.Capability = channel.DRMPlayback
	} else if row.Token != nil && *row.Token != "" {
		c.Capability = channel.Unsupported
	} else if err == nil && c.StreamType == channel.HLS && row.License != nil && row.LicenseToken != nil && row.Token != nil && row.Type == "FAST" && row.State != nil && row.State.ID == 2 {
		// Catalog-qualified clear candidate, based on explicit provider protection
		// fields and the independently observed anonymous HLS delivery protocol.
		// Resolve rechecks both current metadata and the chosen HLS media manifest.
		c.DRM = channel.Clear
		c.Capability = channel.ClearDirect
		if needsRelay(playback) {
			c.Capability = channel.ClearRelayOnly
		}
	}
	for _, bound := range []struct {
		value string
		start bool
	}{{row.Start, true}, {row.End, false}} {
		if bound.value == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, bound.value)
		if err != nil {
			return catalogEntry{}, ErrCatalog
		}
		if (bound.start && now.Before(t)) || (!bound.start && !now.Before(t)) {
			c.Capability = channel.Unsupported
		}
	}
	if c.Validate() != nil {
		return catalogEntry{}, ErrCatalog
	}
	// Fingerprint excludes LastSeen, all URL query values and all unused fields.
	// Delivery origin/path changes matter, but rotating ad/signature values don't.
	identity := c
	identity.LastSeen = time.Time{}
	u, _ := url.Parse(row.URLs[0])
	delivery := ""
	if u != nil {
		delivery = u.Scheme + "://" + u.Host + u.EscapedPath()
	}
	b, _ := json.Marshal(struct {
		Channel  channel.Channel
		Delivery string
	}{identity, delivery})
	digest := sha256.Sum256(b)
	c.Fingerprint = hex.EncodeToString(digest[:])
	return catalogEntry{channel: c, playback: playback}, nil
}

func decodeEnvelope(body []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(body))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return nil, ErrCatalog
	}
	result := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, ErrCatalog
		}
		key, ok := token.(string)
		if !ok {
			return nil, ErrCatalog
		}
		if _, exists := result[key]; exists {
			return nil, ErrCatalog
		}
		switch key {
		case "channels", "currentCategory", "currentChannelStationId", "channelIds", "next", "nextPage", "nextPageToken", "continuationToken", "cursor", "hasMore", "total", "totalCount":
		default:
			return nil, ErrCatalog // unknown envelope may change completeness semantics
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, ErrCatalog
		}
		result[key] = value
	}
	if _, err := d.Token(); err != nil {
		return nil, ErrCatalog
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, ErrCatalog
	}
	return result, nil
}

func decodeRows(raw []byte) ([]upstreamChannel, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	if token, err := d.Token(); err != nil || token != json.Delim('[') {
		return nil, ErrCatalog
	}
	rows := make([]upstreamChannel, 0)
	for d.More() {
		if len(rows) >= maxChannels {
			return nil, ErrCatalog
		}
		var row upstreamChannel
		if d.Decode(&row) != nil {
			return nil, ErrCatalog
		}
		rows = append(rows, row)
	}
	if _, err := d.Token(); err != nil || len(rows) == 0 {
		return nil, ErrCatalog
	}
	return rows, nil
}

// Expand only decoded query values, leaving untouched signed/opaque values byte
// for byte. Never unescape an entire URL or substitute a hostname/path macro.
func expand(raw string) (string, error) {
	u, err := publicURL(raw)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(u.Path, "{}") {
		return "", ErrUnavailable
	}
	parts := strings.Split(u.RawQuery, "&")
	for i, part := range parts {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil {
			return "", ErrUnavailable
		}
		if !strings.ContainsAny(decoded, "{}") {
			continue
		}
		for strings.Contains(decoded, "{") {
			start := strings.IndexByte(decoded, '{')
			end := strings.IndexByte(decoded[start:], '}')
			if end < 0 {
				return "", ErrUnavailable
			}
			end += start
			name := decoded[start+1 : end]
			replacement, ok := macro(name)
			if !ok {
				return "", ErrUnavailable
			}
			decoded = decoded[:start] + replacement + decoded[end+1:]
		}
		if strings.Contains(decoded, "}") {
			return "", ErrUnavailable
		}
		parts[i] = key + "=" + url.QueryEscape(decoded)
	}
	u.RawQuery = strings.Join(parts, "&")
	return u.String(), nil
}

func macro(name string) (string, bool) {
	switch name {
	case "ADID", "TV_AD_ID":
		return "00000000-0000-0000-0000-000000000000", true
	case "LMT", "DNT", "IS_LAT":
		return "1", true
	case "APP_NAME", "DEVICE_MAKE", "DEVICE_MODEL":
		return "ArixTV", true
	case "APP_BUNDLE":
		return "org.arixtv", true
	case "APP_VERSION":
		return "1", true
	case "DEVICE_TYPE":
		return "connected_tv", true
	case "DOMAIN":
		return "https://github.com/Joseph-ink/ArixTV", true
	case "IFATYPE", "IDTYPE":
		return "unknown", true
	case "TARGETOPT":
		return "False", true
	case "USPRIVACY":
		return "1---", true
	case "WIDTH":
		return "1920", true
	case "HEIGHT":
		return "1080", true
	case "COPPA", "SKIPPABLE", "CONTENT_LIVESTREAM":
		return "1", true
	case "APP_STORE_URL", "AD_POS", "CHANNEL_NAME", "GDPR", "GDPR_CONSENT", "GPP", "GPP_SID", "IC", "INV_PARTNER_DOMAIN", "IU", "NETWORK_NAME", "POD_AD_SLOT", "POD_AD_SLOTS", "PRODQ", "SCHAIN", "VIZIO_NETWORK_NAME":
		return "", true
	default:
		return "", false
	}
}
