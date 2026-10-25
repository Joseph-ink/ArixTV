package store

import (
	"encoding/json"
	"errors"
)

func (s *Store) writeSnapshot(path string, snapshot Snapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil || len(data)+1 > maxSnapshotBytes {
		return errors.New("snapshot exceeds persistence limit")
	}
	return s.writeJSON(path, snapshot)
}
