package httpserver

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"glipz.io/backend/internal/config"
	"glipz.io/backend/internal/repo"
)

type securityDiscoveryTransport func(*http.Request) (*http.Response, error)

func (f securityDiscoveryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Opt in against a migrated disposable local DB and a separate Redis DB:
// GLIPZ_SECURITY_TEST_DATABASE_URL=postgres://.../glipz?sslmode=disable
// GLIPZ_SECURITY_TEST_REDIS_URL=redis://127.0.0.1:6379/15
// Discovery is intercepted: this test never sends federation traffic externally.
func TestFederationOwnershipIntegration(t *testing.T) {
	dbURL, redisURL := os.Getenv("GLIPZ_SECURITY_TEST_DATABASE_URL"), os.Getenv("GLIPZ_SECURITY_TEST_REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("requires explicit local security-test DB and Redis URLs")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	ropts, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(ropts)
	t.Cleanup(func() { _ = rdb.Close() })
	s := &Server{db: repo.New(pool), rdb: rdb, cfg: config.Config{GlipzProtocolPublicOrigin: "https://local.example.test"}}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	previousHTTP := federationHTTP
	federationHTTP = &http.Client{Transport: securityDiscoveryTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "8.8.8.8" && req.URL.Host != "1.1.1.1" {
			return nil, fmt.Errorf("unexpected test discovery host")
		}
		origin := "https://" + req.URL.Host
		disc := federationAccountDiscovery{Server: federationServerDiscovery{ProtocolVersion: federationProtocolVersion, Host: req.URL.Host, Origin: origin, KeyID: origin + "/key", PublicKey: base64.StdEncoding.EncodeToString(pub), EventsURL: origin + "/federation/events", FollowURL: origin + "/federation/follow", UnfollowURL: origin + "/federation/unfollow"}}
		body, _ := json.Marshal(disc)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{"Content-Type": {"application/json"}}}, nil
	})}
	t.Cleanup(func() { federationHTTP = previousHTTP })
	owner := "security" + strings.ReplaceAll(uuid.NewString(), "-", "") + "@8.8.8.8"
	other := "other" + owner
	foreign := strings.Replace(owner, "8.8.8.8", "1.1.1.1", 1)
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM federation_remote_accounts WHERE current_acct = ANY($1::text[])`, []string{owner, other, foreign})
		if err != nil {
			t.Error("account cleanup", err)
		}
	})
	send := func(ev federationEventEnvelope, major int) int {
		if ev.EventID == "" {
			ev.EventID = uuid.NewString()
		}
		body, err := json.Marshal(ev)
		if err != nil {
			t.Error(err)
			return 0
		}
		host := strings.Split(ev.Author.Acct, "@")[1]
		key, nonce := "https://"+host+"/key", uuid.NewString()
		ts := time.Now().UTC().Format(time.RFC3339)
		req := httptest.NewRequest("POST", "/federation/events", bytes.NewReader(body))
		for name, value := range map[string]string{"X-Glipz-Instance": host, "X-Glipz-Key-Id": key, "X-Glipz-Protocol-Version": fmt.Sprintf("%s/%d", federationProtocolName, major), "X-Glipz-Timestamp": ts, "X-Glipz-Nonce": nonce, "X-Glipz-Signature": base64.StdEncoding.EncodeToString(ed25519.Sign(priv, federationSignatureMessage("POST", "/federation/events", ts, nonce, body, major)))} {
			req.Header.Set(name, value)
		}
		rec := httptest.NewRecorder()
		s.handleFederationEventInbound(rec, req)
		t.Cleanup(func() {
			_ = rdb.Del(context.Background(), federationNonceRedisKey(key, nonce), federationEventRedisKey(key, ev.EventID)).Err()
		})
		return rec.Code
	}
	newEvent := func(acct, kind, object string) federationEventEnvelope {
		return federationEventEnvelope{V: 3, Kind: kind, Author: federationEventAuthor{Acct: acct}, Post: &federationEventPost{URL: object, Caption: "original #original", MediaType: "none", PublishedAt: time.Now().UTC().Format(time.RFC3339), Poll: &federationEventPoll{EndsAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), Options: []federationEventPollOption{{Position: 1, Label: "original", Votes: 3}}}}}
	}
	newObject := func() string {
		object := "https://8.8.8.8/posts/" + uuid.NewString()
		t.Cleanup(func() {
			if _, err := pool.Exec(context.Background(), `DELETE FROM federation_incoming_posts WHERE object_iri = $1`, object); err != nil {
				t.Error("post cleanup", err)
			}
		})
		return object
	}
	snapshot := func(object string) string {
		var caption, label string
		var deleted bool
		var likes, votes int64
		err := pool.QueryRow(ctx, `SELECT f.caption_text, f.deleted_at IS NOT NULL, f.like_count, COALESCE(o.label, ''), COALESCE(o.votes, 0)
		 FROM federation_incoming_posts f LEFT JOIN federation_incoming_post_poll_options o ON o.federation_incoming_post_id = f.id WHERE f.object_iri = $1`, object).Scan(&caption, &deleted, &likes, &label, &votes)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%s|%v|%d|%s|%d", caption, deleted, likes, label, votes)
	}
	object := newObject()
	if code := send(newEvent(owner, "post_created", object), 3); code != 200 {
		t.Fatalf("owner create: %d", code)
	}
	baseline := snapshot(object)
	accountSnapshot := func() string {
		var value string
		if err := pool.QueryRow(ctx, `SELECT current_acct || '|' || profile_url || '|' || public_key FROM federation_remote_accounts WHERE portable_id = $1`, repo.LegacyPortableIDForAcct(owner)).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	accountBaseline := accountSnapshot()
	for _, kind := range []string{"post_created", "post_updated", "post_deleted", "poll_tally_updated", "post_liked"} {
		forged := newEvent(foreign, kind, object)
		forged.Author.ID = repo.LegacyPortableIDForAcct(owner)
		forged.Author.ProfileURL, forged.Author.PublicKey = "https://1.1.1.1/forged", "forged-key"
		if code := send(forged, 3); code != 403 {
			t.Fatal("forged portable identity accepted", kind, code)
		}
		if accountSnapshot() != accountBaseline || snapshot(object) != baseline {
			t.Fatal("forbidden event modified account or post")
		}
	}
	forgedObject := newObject()
	forged := newEvent(foreign, "post_created", forgedObject)
	forged.Author.ID = repo.LegacyPortableIDForAcct(owner)
	if code := send(forged, 3); code != 403 {
		t.Fatal("new post claimed another portable identity", code)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM federation_incoming_posts WHERE object_iri = $1`, forgedObject).Scan(&count); err != nil || count != 0 {
		t.Fatal("forbidden create was not rolled back", err)
	}
	if accountSnapshot() != accountBaseline {
		t.Fatal("portable identity was overwritten")
	}

	sub := rdb.Subscribe(ctx, redisFeedGlobal)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	for _, major := range []int{2, 3} {
		for _, actor := range []string{other, foreign} {
			for _, kind := range []string{"post_created", "repost_created", "post_updated", "post_deleted", "poll_tally_updated"} {
				ev := newEvent(actor, kind, object)
				ev.Post.Caption, ev.Post.LikeCount, ev.Post.Poll.Options[0].Votes = "tampered #tampered", 999, 999
				if code := send(ev, major); code != 403 {
					t.Fatalf("%s v%d rejected with %d, want 403", kind, major, code)
				}
				if snapshot(object) != baseline {
					t.Fatalf("%s modified victim data", kind)
				}
			}
		}
	}
	noEventCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if msg, err := sub.ReceiveMessage(noEventCtx); err == nil {
		t.Fatalf("rejected event published a notification: %s", msg.Payload)
	}
	for _, kind := range []string{"post_updated", "post_deleted", "poll_tally_updated"} {
		if code := send(newEvent(owner, kind, newObject()), 3); code != 200 {
			t.Fatalf("missing target %s: %d", kind, code)
		}
	}
	// A DB failure while replacing the poll must roll back the caption/hashtags.
	broken := newEvent(owner, "post_updated", object)
	broken.Post.Caption = "must roll back #rollback"
	broken.Author.ProfileURL = "https://8.8.8.8/must-roll-back"
	broken.Post.Poll.Options = append(broken.Post.Poll.Options, broken.Post.Poll.Options[0])
	if code := send(broken, 3); code != 500 {
		t.Fatalf("invalid poll: %d", code)
	}
	if snapshot(object) != baseline || accountSnapshot() != accountBaseline {
		t.Fatal("poll failure did not roll back post/account")
	}
	update := newEvent(owner, "post_updated", object)
	update.EventID, update.Post.Caption = uuid.NewString(), "authorized #updated"
	if code := send(update, 3); code != 200 {
		t.Fatal("owner update", code)
	}
	updated := snapshot(object)
	update.Post.Caption = "replay must not apply"
	if code := send(update, 3); code != 200 || snapshot(object) != updated {
		t.Fatal("event replay was not idempotent")
	}
	// Same-instance likes come from another actor and must still be accepted.
	if code := send(newEvent(other, "post_liked", object), 3); code != 200 {
		t.Fatal("valid relayed like", code)
	}
	if code := send(newEvent(foreign, "post_liked", object), 3); code != 403 {
		t.Fatal("foreign total overwrite accepted", code)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if code := send(newEvent(owner, "post_updated", object), 3); code != 200 {
				t.Errorf("concurrent owner update: %d", code)
			}
		})
	}
	wg.Wait()
	// A verified move updates the stored actor; portable IDs cannot grant access.
	move := newEvent(owner, "account_moved", object)
	move.Move = &federationAccountMove{PortableID: repo.LegacyPortableIDForAcct(owner), OldAcct: owner, NewAcct: other}
	invalidMove := move
	invalidMove.Author.Acct = other
	if code := send(invalidMove, 3); code != 403 {
		t.Fatal("another author moved victim account", code)
	}
	if code := send(move, 3); code != 200 {
		t.Fatal("verified move failed", code)
	}
	movedAccount := accountSnapshot()
	move.Move.ProfileURL = "https://8.8.8.8/former-owner-must-not-overwrite"
	if code := send(move, 3); code != 200 {
		t.Fatal("repeated verified move failed", code)
	}
	if accountSnapshot() != movedAccount {
		t.Fatal("former owner changed metadata through a repeated move")
	}
	if code := send(newEvent(owner, "post_updated", object), 3); code != 403 {
		t.Fatal("old owner accepted after move", code)
	}
	if code := send(newEvent(other, "post_updated", object), 3); code != 200 {
		t.Fatal("new stored owner rejected", code)
	}
	if code := send(newEvent(other, "post_deleted", object), 3); code != 200 {
		t.Fatal("owner delete", code)
	}
	deleted := snapshot(object)
	if code := send(newEvent(other, "post_created", object), 3); code != 200 || snapshot(object) != deleted {
		t.Fatal("tombstone resurrected")
	}
	// Conflicting first creates cannot overwrite the winner's ownership.
	raceObject := newObject()
	codes := make(chan int, 2)
	for _, actor := range []string{owner, other} {
		wg.Go(func() { codes <- send(newEvent(actor, "post_created", raceObject), 3) })
	}
	wg.Wait()
	a, b := <-codes, <-codes
	if !((a == 200 && b == 403) || (a == 403 && b == 200)) {
		t.Fatalf("create collision: %d %d", a, b)
	}
}
