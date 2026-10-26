package server

import (
	"encoding/base64"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

type route struct{ class, secret, provider, channel string }

func component(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
			return false
		}
	}
	return true
}

// No ServeMux or path.Clean: malformed secret-bearing paths never redirect.
func parseRoute(r *http.Request, prefix string) (route, bool) {
	u := r.URL
	if u == nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || len(u.Path) > 2048 || u.EscapedPath() != u.Path || strings.Contains(u.Path, "%") {
		return route{}, false
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if !strings.HasPrefix(u.Path, "/") {
		return route{}, false
	}
	for _, part := range parts {
		if !component(part) {
			return route{}, false
		}
	}
	if u.Path == "/healthz" {
		return route{class: "health"}, true
	}
	if strings.HasPrefix(u.Path, prefix+"/s/") {
		parts = strings.Split(strings.TrimPrefix(u.Path, prefix+"/s/"), "/")
		if len(parts) == 2 && parts[1] == "playlist.m3u" {
			return route{class: "playlist", secret: parts[0]}, true
		}
		if len(parts) == 4 && parts[1] == "play" {
			return route{class: "resolve", secret: parts[0], provider: parts[2], channel: parts[3]}, true
		}
	}
	if len(parts) == 5 && parts[0] == "internal" && parts[2] == "play" {
		return route{class: "internal", secret: parts[1], provider: parts[3], channel: parts[4]}, true
	}
	return route{}, false
}

func secretSyntax(secret, prefix string) bool {
	if len(secret) != 47 || !strings.HasPrefix(secret, prefix) {
		return false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(secret[4:])
	return err == nil && len(b) == 32
}

// Ordinary ASCII IDs remain readable. Other opaque IDs (including slash IDs)
// use a reversible '~'+base64url component, never percent-encoded delimiters.
func channelPath(id string) string {
	if component(id) && !strings.HasPrefix(id, "~") {
		return id
	}
	return "~" + base64.RawURLEncoding.EncodeToString([]byte(id))
}
func channelID(part string) (string, bool) {
	id := part
	if strings.HasPrefix(part, "~") {
		b, err := base64.RawURLEncoding.Strict().DecodeString(part[1:])
		if err != nil {
			return "", false
		}
		id = string(b)
	}
	if len(id) == 0 || len(id) > 256 || !utf8.ValidString(id) || strings.ContainsFunc(id, unicode.IsControl) || channelPath(id) != part {
		return "", false
	}
	return id, true
}
