package httpserver

import (
	"hash/fnv"
	"sync"
	"time"
)

type feedCache struct {
	mu      sync.Mutex
	expires time.Time
	items   []feedItem
}

func (c *feedCache) get(now time.Time) ([]feedItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 || now.After(c.expires) {
		return nil, false
	}
	out := append([]feedItem(nil), c.items...)
	return out, true
}

func (c *feedCache) set(now time.Time, ttl time.Duration, items []feedItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expires = now.Add(ttl)
	c.items = append([]feedItem(nil), items...)
}

type responseCacheEntry struct {
	expires time.Time
	value   any
	used    uint64
}

type responseCache struct {
	mu      sync.Mutex
	entries map[string]responseCacheEntry
	locks   [64]sync.Mutex
	clock   uint64
}

func (c *responseCache) get(now time.Time, key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		return nil, false
	}
	ent, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !now.Before(ent.expires) {
		delete(c.entries, key)
		return nil, false
	}
	c.clock++
	ent.used = c.clock
	c.entries[key] = ent
	return ent.value, true
}

func (c *responseCache) set(now time.Time, key string, ttl time.Duration, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]responseCacheEntry{}
	}
	for k, ent := range c.entries {
		if !now.Before(ent.expires) {
			delete(c.entries, k)
		}
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= 256 {
		var oldest string
		var used uint64 = ^uint64(0)
		for k, ent := range c.entries {
			if ent.used < used {
				oldest, used = k, ent.used
			}
		}
		delete(c.entries, oldest)
	}
	c.clock++
	c.entries[key] = responseCacheEntry{expires: now.Add(ttl), value: value, used: c.clock}
}

func (c *responseCache) keyLock(key string) *sync.Mutex {
	// Fixed stripes bound lock memory even for arbitrarily many distinct keys.
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return &c.locks[h.Sum64()%uint64(len(c.locks))]
}

func (c *responseCache) getOrLoad(now time.Time, key string, ttl time.Duration, load func() (any, error)) (any, bool, error) {
	if v, ok := c.get(now, key); ok {
		return v, true, nil
	}
	l := c.keyLock(key)
	l.Lock()
	defer l.Unlock()
	if v, ok := c.get(time.Now(), key); ok {
		return v, true, nil
	}
	v, err := load()
	if err != nil {
		return nil, false, err
	}
	c.set(time.Now(), key, ttl, v)
	return v, false, nil
}
