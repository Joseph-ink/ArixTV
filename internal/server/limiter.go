package server

import (
	"net/netip"
	"sync"
	"time"

	"github.com/Joseph-ink/ArixTV/internal/config"
)

const (
	preauth      = 0
	playlistRate = 1
	resolveRate  = 2
)

type quota struct {
	tokens float64
	at     time.Time
}
type bucket struct {
	quotas [3]quota
	seen   time.Time
}
type limiter struct {
	mu      sync.Mutex
	clients map[netip.Addr]*bucket
	rates   [3]float64
	bursts  [3]float64
	max     int
	ttl     time.Duration
}

func newLimiter(c config.HTTPConfig) *limiter {
	ttl, _ := time.ParseDuration(c.RateClientTTL)
	return &limiter{clients: make(map[netip.Addr]*bucket), rates: [3]float64{float64(max(c.PlaylistPerMinute, c.ResolvePerMinute)*4) / 60, float64(c.PlaylistPerMinute) / 60, float64(c.ResolvePerMinute) / 60}, bursts: [3]float64{float64(c.RateBurst * 4), float64(c.RateBurst), float64(c.RateBurst)}, max: c.RateMaxClients, ttl: ttl}
}
func (l *limiter) allow(ip netip.Addr, class int, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b := l.clients[ip]
	if b == nil {
		if len(l.clients) >= l.max {
			for key, value := range l.clients {
				if now.Sub(value.seen) >= l.ttl {
					delete(l.clients, key)
				}
			}
		}
		if len(l.clients) >= l.max {
			return false
		}
		b = &bucket{seen: now}
		for i := range b.quotas {
			b.quotas[i] = quota{tokens: l.bursts[i], at: now}
		}
		l.clients[ip] = b
	}
	b.seen = now
	q := &b.quotas[class]
	if now.After(q.at) {
		q.tokens = min(l.bursts[class], q.tokens+now.Sub(q.at).Seconds()*l.rates[class])
		q.at = now
	}
	if q.tokens < 1 {
		return false
	}
	q.tokens--
	return true
}
