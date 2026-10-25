package auth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/filelock"
	"github.com/Joseph-ink/ArixTV/internal/logsafe"
)

func TestTokenEntropyAndRepresentation(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if a.Reveal() == b.Reveal() {
		t.Fatal("token collision")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(a.Reveal()[4:])
	if err != nil || len(decoded) != 32 {
		t.Fatal("token not 256 bits")
	}
	for _, formatted := range []string{fmt.Sprint(a), fmt.Sprintf("%#v", a), fmt.Sprintf("%+v", a)} {
		if strings.Contains(formatted, a.Reveal()) {
			t.Fatal("plaintext formatted")
		}
	}
	data, _ := json.Marshal(a)
	if strings.Contains(string(data), a.Reveal()) {
		t.Fatal("plaintext JSON")
	}
	var buf bytes.Buffer
	logsafe.New(&buf).Info("token created", "result", a)
	if strings.Contains(buf.String(), a.Reveal()) {
		t.Fatal("plaintext in log")
	}
	if _, err := generate(strings.NewReader("short")); err == nil {
		t.Fatal("entropy error ignored")
	}
}

func TestTokenLifecycle(t *testing.T) {
	s := NewStore(t.TempDir())
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	a, record, err := s.Create("living room")
	if err != nil {
		t.Fatal(err)
	}
	if record.Hash == a.Reveal() || len(record.Hash) != 64 {
		t.Fatal("token hash missing")
	}
	if ok, err := s.Verify(a.Reveal()); err != nil || !ok {
		t.Fatal("created token rejected", err)
	}
	other, _ := Generate()
	if ok, err := s.Verify(other.Reveal()); err != nil || ok {
		t.Fatal("wrong token accepted")
	}
	if _, _, err := s.Create("living room"); err == nil {
		t.Fatal("duplicate name accepted")
	}
	now = now.Add(time.Minute)
	b, rotated, err := s.Rotate("living room")
	if err != nil {
		t.Fatal(err)
	}
	if rotated.CreatedAt != record.CreatedAt || rotated.RotatedAt == nil {
		t.Fatal("rotation lost lifecycle metadata")
	}
	if ok, _ := s.Verify(a.Reveal()); ok {
		t.Fatal("old token survived rotation")
	}
	if ok, _ := s.Verify(b.Reveal()); !ok {
		t.Fatal("new token rejected")
	}
	data, _ := os.ReadFile(s.path())
	if bytes.Contains(data, []byte(a.Reveal())) || bytes.Contains(data, []byte(b.Reveal())) {
		t.Fatal("plaintext persisted")
	}
	info, _ := os.Stat(s.path())
	if info.Mode().Perm() != 0600 {
		t.Fatal("token file permissions")
	}
	for _, name := range []string{"\xff", "\n", ""} {
		if _, _, err := s.Create(name); err == nil {
			t.Fatal("invalid name accepted")
		}
	}
	if err := s.Revoke("living room"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Verify(b.Reveal()); ok {
		t.Fatal("revoked token accepted")
	}
	if err := s.Revoke("living room"); err != nil {
		t.Fatal("revoke not idempotent")
	}
	if _, _, err := s.Rotate("living room"); err == nil {
		t.Fatal("revoked token reactivated")
	}
	listed, err := s.List()
	if err != nil || len(listed) != 1 || listed[0].RevokedAt == nil {
		t.Fatal("list state", err)
	}
	if err := s.Revoke("unknown"); err == nil {
		t.Fatal("unknown token revocation succeeded")
	}
	for _, value := range []string{"", "PLACEHOLDER", strings.Repeat("a", 47), "atv_" + strings.Repeat("!", 43)} {
		if ok, _ := s.Verify(value); ok {
			t.Fatal("invalid syntax verified")
		}
	}
}

func TestMutationFailurePreservesToken(t *testing.T) {
	s := NewStore(t.TempDir())
	secret, _, err := s.Create("example")
	if err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(s.path())
	entropy, err := base64.RawURLEncoding.DecodeString(secret.Reveal()[4:])
	if err != nil {
		t.Fatal(err)
	}
	s.random = bytes.NewReader(entropy)
	if _, _, err := s.Rotate("example"); err == nil {
		t.Fatal("rotation reused old token")
	}
	s.random = bytes.NewReader(bytes.Repeat([]byte{7}, 32))
	s.writeJSON = func(string, any) error { return errors.New("injected precommit failure") }
	if result, _, err := s.Rotate("example"); err == nil || result.Reveal() != "" {
		t.Fatal("failed rotation exposed usable result")
	}
	if err := s.Revoke("example"); err == nil {
		t.Fatal("injected failure ignored")
	}
	current, _ := os.ReadFile(s.path())
	if !bytes.Equal(old, current) {
		t.Fatal("mutation failure changed state")
	}
	if ok, _ := s.Verify(secret.Reveal()); !ok {
		t.Fatal("failure revoked LKG token")
	}
	s.random = io.LimitReader(strings.NewReader(""), 0)
	if _, _, err := s.Create("another"); err == nil {
		t.Fatal("entropy failure ignored")
	}
	lock, err := filelock.Acquire(s.dir + "/tokens.lock")
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, _, err := s.Rotate("example"); !errors.Is(err, filelock.ErrBusy) {
		t.Fatalf("mutation ignored process lock: %v", err)
	}
}

func TestCorruptTokenStateFailsClosed(t *testing.T) {
	s := NewStore(t.TempDir())
	secret, _, err := s.Create("example")
	if err != nil {
		t.Fatal(err)
	}
	valid, _ := os.ReadFile(s.path())
	for _, bad := range [][]byte{[]byte(`{"schema_version":2,"records":[]}`), []byte(`{"schema_version":1,"records":null}`), []byte(`{"schema_version":1,"records":[{"name":"x","sha256":"bad"}]}`), []byte(`{`), append(valid, []byte(` {}`)...)} {
		os.WriteFile(s.path(), bad, 0600)
		if ok, err := s.Verify(secret.Reveal()); ok || err == nil {
			t.Fatal("corrupt state granted access")
		}
		if _, _, err := s.Create("another"); err == nil {
			t.Fatal("corrupt state overwritten")
		}
		now, _ := os.ReadFile(s.path())
		if !bytes.Equal(now, bad) {
			t.Fatal("corrupt file reset silently")
		}
	}
	if _, err := NewStore("").List(); err == nil {
		t.Fatal("empty state path accepted")
	}
}
