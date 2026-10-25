package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/atomicfile"
	"github.com/Joseph-ink/ArixTV/internal/filelock"
	"github.com/Joseph-ink/ArixTV/internal/statefile"
)

const maxTokens = 1024

type tokenFile struct {
	Version int      `json:"schema_version"`
	Records []Record `json:"records"`
}
type Store struct {
	dir       string
	now       func() time.Time
	random    io.Reader
	writeJSON func(string, any) error
}

// NewStore only binds paths. It never creates/replaces state implicitly.
func NewStore(stateDir string) *Store {
	return &Store{dir: stateDir, now: time.Now, random: rand.Reader, writeJSON: atomicfile.WriteJSON}
}
func (s *Store) path() string { return filepath.Join(s.dir, "tokens.json") }

func (s *Store) read() (tokenFile, error) {
	var f tokenFile
	if strings.TrimSpace(s.dir) == "" {
		return f, errors.New("state directory required")
	}
	if err := statefile.ReadJSON(s.path(), 1<<20, &f); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return tokenFile{Version: 1, Records: []Record{}}, nil
		}
		return f, err
	}
	if f.Version != 1 || f.Records == nil || len(f.Records) > maxTokens {
		return f, errors.New("invalid token state version or size")
	}
	names := map[string]bool{}
	hashes := map[string]bool{}
	for _, r := range f.Records {
		if err := r.validate(); err != nil {
			return f, err
		}
		if names[r.Name] || hashes[r.Hash] {
			return f, errors.New("duplicate token record")
		}
		names[r.Name] = true
		hashes[r.Hash] = true
	}
	return f, nil
}

func (s *Store) List() ([]Record, error) {
	f, err := s.read()
	if err != nil {
		return nil, err
	}
	sort.Slice(f.Records, func(i, j int) bool { return f.Records[i].Name < f.Records[j].Name })
	return f.Records, nil
}

// Verify hashes once and scans every record, with constant-time digest compares.
// Invalid/corrupt state is an error; a caller must fail closed, never allow access.
func (s *Store) Verify(token string) (bool, error) {
	if len(token) != 47 || !strings.HasPrefix(token, "atv_") {
		return false, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token[4:])
	if err != nil || len(b) != 32 {
		return false, nil
	}
	f, err := s.read()
	if err != nil {
		return false, err
	}
	h := digest(token)
	matched := 0
	for _, r := range f.Records {
		matched |= r.matches(h)
	}
	return matched == 1, nil
}

func (s *Store) Create(name string) (Secret, Record, error) { return s.change(name, false) }
func (s *Store) Rotate(name string) (Secret, Record, error) { return s.change(name, true) }

func (s *Store) change(name string, rotate bool) (Secret, Record, error) {
	if !validName(name) {
		return Secret{}, Record{}, errors.New("invalid token name")
	}
	lock, err := s.lock()
	if err != nil {
		return Secret{}, Record{}, err
	}
	defer lock.Close()
	f, err := s.read()
	if err != nil {
		return Secret{}, Record{}, err
	}
	index := -1
	for i, r := range f.Records {
		if r.Name == name {
			index = i
		}
	}
	if (!rotate && index >= 0) || (rotate && (index < 0 || f.Records[index].RevokedAt != nil)) {
		return Secret{}, Record{}, errors.New("token name already exists or is not active")
	}
	if !rotate && len(f.Records) >= maxTokens {
		return Secret{}, Record{}, errors.New("token limit reached")
	}
	secret, err := generate(s.random)
	if err != nil {
		return Secret{}, Record{}, err
	}
	now := s.now().UTC()
	record := newRecord(name, secret, now)
	if rotate {
		if record.Hash == f.Records[index].Hash {
			return Secret{}, Record{}, errors.New("rotation generated the previous token")
		}
		record.CreatedAt = f.Records[index].CreatedAt
		record.RotatedAt = &now
		f.Records[index] = record
	} else {
		f.Records = append(f.Records, record)
	}
	if err := record.validate(); err != nil {
		return Secret{}, Record{}, err
	}
	// Duplicate entropy is extraordinarily unlikely, but do not persist it.
	for _, r := range f.Records {
		if r.Name != name && r.Hash == record.Hash {
			return Secret{}, Record{}, errors.New("token hash collision")
		}
	}
	if err := s.writeJSON(s.path(), f); err != nil {
		return Secret{}, Record{}, err
	}
	return secret, record, nil
}

func (s *Store) Revoke(name string) error {
	lock, err := s.lock()
	if err != nil {
		return err
	}
	defer lock.Close()
	f, err := s.read()
	if err != nil {
		return err
	}
	for i, r := range f.Records {
		if r.Name == name {
			if r.RevokedAt != nil {
				return nil
			}
			now := s.now().UTC()
			f.Records[i].RevokedAt = &now
			if err := f.Records[i].validate(); err != nil {
				return err
			}
			return s.writeJSON(s.path(), f)
		}
	}
	return errors.New("token name not found")
}

func (s *Store) lock() (*filelock.Lock, error) {
	if strings.TrimSpace(s.dir) == "" {
		return nil, errors.New("state directory required")
	}
	if err := atomicfile.EnsureDir(s.dir); err != nil {
		return nil, errors.New("cannot create token state directory")
	}
	return filelock.Acquire(filepath.Join(s.dir, "tokens.lock"))
}
