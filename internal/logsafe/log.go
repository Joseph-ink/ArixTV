// Package logsafe provides conservative slog JSON logging. Messages must be
// static templates; secrets belong in named attributes or Secret values.
package logsafe

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const Redacted = "[REDACTED]"

var (
	urlPattern           = regexp.MustCompile(`(?i)https?://[^\s<>"']+`)
	protectedPathPattern = regexp.MustCompile(`(?i)(/(?:s|internal)/)[^/?#\s"']+`)
	tokenPattern         = regexp.MustCompile(`atv_[A-Za-z0-9_-]{43}`)
	headerPattern        = regexp.MustCompile(`(?i)(authorization|proxy-authorization|cookie|set-cookie)\s*[:=][^\r\n]*`)
	credentialPattern    = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/-]+=*`)
	assignmentPattern    = regexp.MustCompile(`(?i)([A-Za-z0-9_-]*(?:token|secret|session|password|signature|credential|api_key)[A-Za-z0-9_-]*\s*[:=]\s*)[^\s,;]+`)
)

func SensitiveKey(key string) bool {
	k := strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
	for _, part := range []string{"authorization", "cookie", "token", "secret", "session", "password", "credential", "signature", "apikey", "headers"} {
		if strings.Contains(k, part) {
			return true
		}
	}
	return k == "key"
}

// Text strips URL paths, all query values, credentials, fragments, known ArixTV
// tokens and header/credential assignments. This intentionally favors omission.
func Text(text string) string {
	text = urlPattern.ReplaceAllStringFunc(text, func(raw string) string {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return "[redacted URL]"
		}
		u.User = nil
		u.Path = "/[REDACTED]"
		u.RawPath = ""
		u.Fragment = ""
		u.RawFragment = ""
		if u.RawQuery != "" || u.ForceQuery {
			u.RawQuery = "REDACTED"
			u.ForceQuery = false
		}
		return u.String()
	})
	text = tokenPattern.ReplaceAllString(text, Redacted)
	text = protectedPathPattern.ReplaceAllString(text, "${1}"+Redacted)
	text = headerPattern.ReplaceAllString(text, "header="+Redacted)
	text = credentialPattern.ReplaceAllString(text, "Bearer "+Redacted)
	return assignmentPattern.ReplaceAllString(text, "credential="+Redacted)
}

type binding struct {
	groups []string
	attr   slog.Attr
}
type handler struct {
	base   slog.Handler
	groups []string
	attrs  []binding
}

func New(w io.Writer) *slog.Logger {
	return slog.New(&handler{base: slog.NewJSONHandler(w, nil)})
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	n := *h
	n.attrs = append([]binding(nil), h.attrs...)
	for _, a := range attrs {
		n.attrs = append(n.attrs, binding{groups: append([]string(nil), h.groups...), attr: sanitize(a)})
	}
	return &n
}
func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	n := *h
	n.groups = append(append([]string(nil), h.groups...), name)
	return &n
}
func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, Text(r.Message), r.PC)
	for _, b := range h.attrs {
		clean.AddAttrs(wrap(b.groups, b.attr))
	}
	r.Attrs(func(a slog.Attr) bool { clean.AddAttrs(wrap(h.groups, sanitize(a))); return true })
	return h.base.Handle(ctx, clean)
}

func wrap(groups []string, a slog.Attr) slog.Attr {
	for i := len(groups) - 1; i >= 0; i-- {
		if SensitiveKey(groups[i]) {
			a = slog.String(Text(groups[i]), Redacted)
		} else {
			a = slog.Attr{Key: Text(groups[i]), Value: slog.GroupValue(a)}
		}
	}
	return a
}

func sanitize(a slog.Attr) slog.Attr {
	key := Text(a.Key)
	if SensitiveKey(a.Key) {
		return slog.String(key, Redacted)
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(key, Text(v.String()))
	case slog.KindGroup:
		attrs := v.Group()
		safe := make([]slog.Attr, 0, len(attrs))
		for _, x := range attrs {
			safe = append(safe, sanitize(x))
		}
		return slog.Attr{Key: key, Value: slog.GroupValue(safe...)}
	case slog.KindAny:
		// Do not serialize unknown structs/maps/errors: their nested fields,
		// String/MarshalJSON methods or headers may expose arbitrary secrets.
		return slog.String(key, Redacted)
	case slog.KindTime:
		return slog.Time(key, v.Time().UTC())
	case slog.KindDuration:
		return slog.Duration(key, time.Duration(v.Duration()))
	default:
		return slog.Attr{Key: key, Value: v}
	}
}
