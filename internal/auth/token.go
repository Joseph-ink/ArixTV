// Package auth implements hashed public token state, not HTTP authentication.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Secret has no exported fields and deliberately redacts formatting/JSON/logging.
// Reveal is the sole explicit plaintext access, intended for creation/rotation.
type Secret struct{ plaintext string }

func (s Secret) Reveal() string             { return s.plaintext }
func (Secret) String() string               { return "[REDACTED]" }
func (Secret) GoString() string             { return "[REDACTED]" }
func (Secret) LogValue() slog.Value         { return slog.StringValue("[REDACTED]") }
func (Secret) MarshalJSON() ([]byte, error) { return json.Marshal("[REDACTED]") }

func Generate() (Secret, error) { return generate(rand.Reader) }
func generate(source io.Reader) (Secret, error) {
	var b [32]byte
	if _, err := io.ReadFull(source, b[:]); err != nil {
		return Secret{}, errors.New("cannot generate token entropy")
	}
	return Secret{plaintext: "atv_" + base64.RawURLEncoding.EncodeToString(b[:])}, nil
}

type Record struct {
	Name      string     `json:"name"`
	Hash      string     `json:"sha256"`
	CreatedAt time.Time  `json:"created_at"`
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func digest(token string) [32]byte { return sha256.Sum256([]byte(token)) }
func newRecord(name string, s Secret, now time.Time) Record {
	h := digest(s.Reveal())
	return Record{Name: name, Hash: hex.EncodeToString(h[:]), CreatedAt: now.UTC()}
}
func validName(name string) bool {
	return utf8.ValidString(name) && strings.TrimSpace(name) != "" && len(name) <= 128 && !strings.ContainsFunc(name, unicode.IsControl)
}
func (r Record) validate() error {
	h, err := hex.DecodeString(r.Hash)
	if !validName(r.Name) || err != nil || len(h) != 32 || r.Hash != hex.EncodeToString(h) || r.CreatedAt.IsZero() {
		return errors.New("invalid token record")
	}
	if r.RotatedAt != nil && r.RotatedAt.Before(r.CreatedAt) || r.RevokedAt != nil && r.RevokedAt.Before(r.CreatedAt) {
		return errors.New("invalid token timestamps")
	}
	return nil
}
func (r Record) matches(hash [32]byte) int {
	expected, err := hex.DecodeString(r.Hash)
	if err != nil || len(expected) != 32 {
		return 0
	}
	match := subtle.ConstantTimeCompare(expected, hash[:])
	if r.RevokedAt != nil {
		match = 0
	}
	return match
}
