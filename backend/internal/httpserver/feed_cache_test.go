package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResponseCacheBoundedLRUAndExpiry(t *testing.T) {
	var c responseCache
	now := time.Now()
	for i := range 256 {
		c.set(now, fmt.Sprint(i), time.Minute, i)
	}
	if _, ok := c.get(now, "0"); !ok {
		t.Fatal("missing entry")
	}
	c.set(now, "new", time.Minute, 256)
	if len(c.entries) != 256 {
		t.Fatal("unbounded cache")
	}
	if _, ok := c.get(now, "1"); ok {
		t.Fatal("least recently used entry retained")
	}
	if _, ok := c.get(now, "0"); !ok {
		t.Fatal("recently used entry evicted")
	}
	if _, ok := c.get(now.Add(time.Minute), "0"); ok {
		t.Fatal("expired entry accepted")
	}
	if _, ok := c.entries["0"]; ok {
		t.Fatal("expired entry retained")
	}
	c.set(now.Add(time.Minute), "fresh", time.Minute, 1)
	if len(c.entries) != 1 {
		t.Fatal("expired entries not purged on insertion")
	}
	for i := range 10000 {
		c.keyLock(fmt.Sprint(i))
	}
	if len(c.locks) != 64 {
		t.Fatal("unbounded lock storage")
	}
}

func TestResponseCacheConcurrentLoadAndFailure(t *testing.T) {
	var c responseCache
	var calls atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 32 {
		wg.Go(func() {
			<-start
			v, _, err := c.getOrLoad(time.Now(), "shared", time.Minute, func() (any, error) { calls.Add(1); return "value", nil })
			if err != nil || v != "value" {
				t.Errorf("load: %v, %v", v, err)
			}
		})
	}
	close(start)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("duplicate same-key loads", calls.Load())
	}
	_, _, err := c.getOrLoad(time.Now(), "failed", time.Minute, func() (any, error) { return nil, errors.New("offline") })
	if err == nil {
		t.Fatal("load failure swallowed")
	}
	if _, ok := c.get(time.Now(), "failed"); ok {
		t.Fatal("failure cached")
	}
	for i := range 1000 {
		wg.Go(func() { c.set(time.Now(), fmt.Sprint(i), time.Minute, i); c.get(time.Now(), fmt.Sprint(i)) })
	}
	wg.Wait()
	if len(c.entries) > 256 {
		t.Fatal("concurrent growth exceeded limit")
	}
}

func TestFeedScopeValidation(t *testing.T) {
	for raw, want := range map[string]string{"": "all", "all": "all", " ALL ": "all", "following": "following", "recommended": "recommended"} {
		got, ok := normalizeFeedScope(raw)
		if !ok || got != want {
			t.Errorf("%q: %q %v", raw, got, ok)
		}
	}
	// A server with no DB must reject arbitrary scopes before cache/DB access,
	// including requests which would otherwise bypass the first-page cache.
	s := &Server{}
	for _, path := range []string{"/feed?scope=unbounded", "/feed?scope=unbounded&offset=10"} {
		req := httptest.NewRequest("GET", path, nil)
		req = req.WithContext(context.WithValue(req.Context(), ctxUserID{}, uuid.New()))
		rec := httptest.NewRecorder()
		s.handleFeed(rec, req)
		if rec.Code != 400 || len(s.userFeedCache.entries) != 0 {
			t.Fatal("invalid scope was not rejected before caching")
		}
	}
}
