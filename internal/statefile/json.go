// Package statefile reads bounded, strict JSON state. Malformed/unknown versions
// are never silently treated as empty state.
package statefile

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
)

func ReadJSON(path string, limit int64, out any) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.ErrNotExist
	}
	if err != nil {
		return errors.New("cannot open state file")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(data)) > limit {
		return errors.New("cannot read state within size limit")
	}
	return Decode(data, out)
}

func Decode(data []byte, out any) error {
	// encoding/json otherwise accepts duplicate keys by taking the last value.
	tokens := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueValue(tokens, 0); err != nil {
		return errors.New("invalid state JSON")
	}
	if _, err := tokens.Token(); err != io.EOF {
		return errors.New("trailing state JSON")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return errors.New("invalid state schema")
	}
	return nil
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("JSON depth exceeded")
	}
	t, err := d.Token()
	if err != nil {
		return err
	}
	switch t {
	case json.Delim('{'):
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate JSON key")
			}
			seen[name] = true
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("invalid object")
		}
	case json.Delim('['):
		for d.More() {
			if err := uniqueValue(d, depth+1); err != nil {
				return err
			}
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("invalid array")
		}
	case json.Delim('}'), json.Delim(']'):
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}
