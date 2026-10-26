package xumo

import (
	"bytes"
	"encoding/json"
	"io"
)

// object rejects duplicate keys and unknown envelopes. Each protocol caller
// names its allowed fields explicitly; unknown guide/description row fields are
// discarded separately. No arbitrary JSON is persisted.
func object(raw []byte, allowed ...string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrProtocol
	}
	fields := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		fields[key] = true
	}
	result := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, ErrProtocol
		}
		key, ok := token.(string)
		if !ok || (len(allowed) > 0 && !fields[key]) {
			return nil, ErrProtocol
		}
		if _, seen := result[key]; seen {
			return nil, ErrProtocol
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, ErrProtocol
		}
		result[key] = value
	}
	if _, err := d.Token(); err != nil {
		return nil, ErrProtocol
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, ErrProtocol
	}
	return result, nil
}
func array[T any](raw []byte, limit int) ([]T, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('[') {
		return nil, ErrProtocol
	}
	result := make([]T, 0)
	for d.More() {
		if len(result) >= limit {
			return nil, ErrProtocol
		}
		var row T
		if d.Decode(&row) != nil {
			return nil, ErrProtocol
		}
		result = append(result, row)
	}
	if _, err := d.Token(); err != nil {
		return nil, ErrProtocol
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, ErrProtocol
	}
	return result, nil
}
