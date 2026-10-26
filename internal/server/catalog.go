package server

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/channel"
	"github.com/Joseph-ink/ArixTV/internal/store"
)

type channelKey struct{ provider, id string }
type catalogView struct {
	channels    map[channelKey]channel.Channel
	ordered     []channel.Channel
	providers   []snapshotHealth
	degraded    bool
	directCount int
}
type snapshotHealth struct {
	Provider     string     `json:"provider"`
	Generation   uint64     `json:"generation"`
	ChannelCount int        `json:"channel_count"`
	LastSuccess  *time.Time `json:"last_success,omitempty"`
	State        string     `json:"state"`
}
type stamp struct {
	size     int64
	modified time.Time
	inode    os.FileInfo
}
type cachedSnapshot struct {
	snapshot store.Snapshot
	stamp    stamp
}
type cachedMetadata struct {
	stamp      stamp
	generation uint64
	state      string
}
type catalog struct {
	mu       sync.Mutex
	view     atomic.Pointer[catalogView]
	store    *store.Store
	dir      string
	stateDir string
	ids      []string
	cache    map[string]cachedSnapshot
	metadata map[string]cachedMetadata
}

func (c *catalog) reload(startup bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := &catalogView{channels: make(map[channelKey]channel.Channel)}
	for _, id := range c.ids {
		path := filepath.Join(c.dir, "providers", id+".json")
		info, err := os.Stat(path)
		old, hadOld := c.cache[id]
		state := "healthy"
		if err == nil && hadOld && sameStamp(old.stamp, info) {
			// No disk parse for unchanged completed snapshots.
		} else if err == nil {
			loaded, loadErr := c.store.Load(id)
			if loadErr == nil {
				total := len(loaded.Channels)
				for provider, cached := range c.cache {
					if provider != id {
						total += len(cached.snapshot.Channels)
					}
				}
				if total > 100000 {
					loadErr = errors.New("catalog channel limit exceeded")
				}
			}
			if loadErr == nil && hadOld && (loaded.Generation < old.snapshot.Generation || loaded.Generation == old.snapshot.Generation && !reflect.DeepEqual(loaded, old.snapshot)) {
				loadErr = errors.New("snapshot generation did not advance")
			}
			if loadErr != nil {
				err = loadErr
			} else {
				// If a rename raced the read, don't cache a stamp; retry next poll.
				after, statErr := os.Stat(path)
				nextStamp := stamp{}
				if statErr == nil && os.SameFile(info, after) && info.ModTime().Equal(after.ModTime()) && info.Size() == after.Size() {
					nextStamp = stamp{size: after.Size(), modified: after.ModTime(), inode: after}
				}
				old = cachedSnapshot{snapshot: loaded, stamp: nextStamp}
				c.cache[id] = old
				hadOld = true
			}
		}
		if err != nil {
			if startup && !errors.Is(err, os.ErrNotExist) {
				return errors.New("cannot load valid provider snapshot")
			}
			state = "missing"
			if hadOld || !errors.Is(err, os.ErrNotExist) {
				state = "reload_failed"
			}
			v.degraded = true
		}
		if state != "reload_failed" {
			generation := uint64(0)
			if hadOld {
				generation = old.snapshot.Generation
			}
			observed, statusErr := c.observation(id, generation)
			if statusErr != nil {
				if startup {
					return errors.New("cannot load valid refresh metadata")
				}
				state = "metadata_invalid"
				v.degraded = true
			} else if observed != "" {
				state = observed
				if state == "failed" || state == "stale" || state == "unknown" {
					v.degraded = true
				}
			}
		}
		h := snapshotHealth{Provider: id, State: state}
		if hadOld {
			s := old.snapshot
			h.Generation = s.Generation
			h.ChannelCount = len(s.Channels)
			at := s.CompletedAt
			h.LastSuccess = &at
			for _, ch := range s.Channels {
				if ch.Capability == channel.ClearDirect {
					v.directCount++
				}
				v.channels[channelKey{id, ch.ProviderChannelID}] = ch
				v.ordered = append(v.ordered, ch)
			}
		}
		v.providers = append(v.providers, h)
	}
	sort.Slice(v.ordered, func(i, j int) bool {
		a, b := v.ordered[i], v.ordered[j]
		if a.ProviderID != b.ProviderID {
			return a.ProviderID < b.ProviderID
		}
		return a.ProviderChannelID < b.ProviderChannelID
	})
	c.view.Store(v)
	if v.degraded {
		return errors.New("one or more snapshots unavailable; valid generations retained")
	}
	return nil
}

func sameStamp(s stamp, info os.FileInfo) bool {
	return s.inode != nil && s.size == info.Size() && s.modified.Equal(info.ModTime()) && os.SameFile(s.inode, info)
}

func (c *catalog) observation(id string, generation uint64) (string, error) {
	path := filepath.Join(c.stateDir, "refresh", id+".json")
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		delete(c.metadata, id)
		return "", nil
	}
	if err != nil {
		return "", errors.New("cannot inspect refresh metadata")
	}
	if old, ok := c.metadata[id]; ok && old.generation == generation && sameStamp(old.stamp, info) {
		return old.state, nil
	}
	m, err := c.store.Status(id)
	if err != nil {
		return "", err
	}
	if m.Generation != generation {
		return "", errors.New("refresh metadata and snapshot changed during reload")
	}
	// Do not cache the stat/read race; the next poll rechecks the observation.
	after, err := os.Stat(path)
	if err == nil && os.SameFile(info, after) && info.ModTime().Equal(after.ModTime()) && info.Size() == after.Size() {
		c.metadata[id] = cachedMetadata{stamp: stamp{size: after.Size(), modified: after.ModTime(), inode: after}, generation: generation, state: string(m.Status)}
	}
	return string(m.Status), nil
}
