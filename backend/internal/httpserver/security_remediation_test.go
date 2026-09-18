package httpserver

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"glipz.io/backend/internal/authjwt"
	"glipz.io/backend/internal/repo"
	"glipz.io/backend/internal/s3client"
)

type testPushStore struct {
	subscriptions []repo.PushSubscription
	removed       atomic.Int64
}

func (m *testPushStore) ListPushSubscriptionsByUser(context.Context, uuid.UUID) ([]repo.PushSubscription, error) {
	return m.subscriptions, nil
}
func (m *testPushStore) DeletePushSubscription(context.Context, uuid.UUID, string) error {
	m.removed.Add(1)
	return nil
}
func (m *testPushStore) DeletePushSubscriptionByEndpoint(context.Context, string) error { return nil }
func (m *testPushStore) MarkPushSubscriptionFailure(context.Context, string, string) error {
	return nil
}
func (m *testPushStore) MarkPushSubscriptionSuccess(context.Context, string) error { return nil }

type testPushTransport func(*http.Request) (*http.Response, error)

func (f testPushTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStoredPushEndpointRejectedAndSendDeadlineHonored(t *testing.T) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	subscription := repo.PushSubscription{Endpoint: "https://127.0.0.1/push", P256DH: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}
	store := &testPushStore{subscriptions: []repo.PushSubscription{subscription}}
	calls := atomic.Int64{}
	s := &Server{pushSubscriptions: store, pushClient: &http.Client{Timeout: time.Second, Transport: testPushTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}}
	s.cfg.WebPushVAPIDPublicKey = public
	s.cfg.WebPushVAPIDPrivateKey = private
	s.cfg.WebPushVAPIDSubject = "mailto:audit@example.test"
	s.sendWebPushToUser(t.Context(), uuid.New(), webPushNotification{})
	if calls.Load() != 0 || store.removed.Load() != 1 {
		t.Fatal("legacy private endpoint was contacted")
	}
	store.subscriptions[0].Endpoint = "https://fcm.googleapis.com/push"
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	start := time.Now()
	s.sendWebPushToUser(ctx, uuid.New(), webPushNotification{})
	if calls.Load() != 1 || time.Since(start) > 500*time.Millisecond {
		t.Fatal("caller deadline was not propagated to push HTTP request")
	}
}

func TestPushQueueBoundsWork(t *testing.T) {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	store := &testPushStore{subscriptions: []repo.PushSubscription{{Endpoint: "https://fcm.googleapis.com/push", P256DH: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}}
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{}, pushWorkers)
	s := &Server{pushSubscriptions: store, pushClient: &http.Client{Transport: testPushTransport(func(r *http.Request) (*http.Response, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
		return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}}
	s.cfg.WebPushVAPIDPublicKey = public
	s.cfg.WebPushVAPIDPrivateKey = private
	s.cfg.WebPushVAPIDSubject = "mailto:audit@example.test"
	for i := 0; i < pushWorkers; i++ {
		s.enqueuePush(uuid.New(), webPushNotification{})
	}
	for i := 0; i < pushWorkers; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("worker not started")
		}
	}
	for i := 0; i < 1000; i++ {
		s.enqueuePush(uuid.New(), webPushNotification{})
	}
	if len(s.push.jobs) != pushQueueSize {
		t.Fatalf("queue has %d pending jobs", len(s.push.jobs))
	}
}

type testMediaStore struct {
	posts      []repo.MediaPostAccess
	public, dm bool
	err        error
}

func (m *testMediaStore) MediaPostAccess(context.Context, string, uuid.UUID) ([]repo.MediaPostAccess, error) {
	return m.posts, m.err
}
func (m *testMediaStore) PublicMediaAsset(context.Context, string) (bool, error) {
	return m.public, m.err
}
func (m *testMediaStore) DMObjectReadable(context.Context, string, uuid.UUID) (bool, error) {
	return m.dm, m.err
}

func TestMediaAuthorizationChangesApplyToExistingURL(t *testing.T) {
	owner := uuid.New()
	key := "uploads/" + owner.String() + "/image.png"
	store, err := s3client.NewLocal(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PutObject(t.Context(), key, "image/png", strings.NewReader("image"), 5); err != nil {
		t.Fatal(err)
	}
	permissions := &testMediaStore{posts: []repo.MediaPostAccess{{ID: uuid.New(), Owner: owner, Readable: true}}}
	s := &Server{s3: store, mediaAccess: permissions}
	router := chi.NewRouter()
	router.Get("/media/*", s.handlePublicMediaObject)
	router.Head("/media/*", s.handlePublicMediaObject)
	for _, test := range []struct {
		name                                 string
		readable, password, membership, fail bool
		want                                 int
	}{
		{name: "public", readable: true, want: 200},
		{name: "changed to private", want: 404},
		{name: "password", readable: true, password: true, want: 404},
		{name: "membership", readable: true, membership: true, want: 404},
		{name: "store unavailable", readable: true, fail: true, want: 503},
	} {
		t.Run(test.name, func(t *testing.T) {
			permissions.posts[0].Readable = test.readable
			permissions.posts[0].HasPassword = test.password
			permissions.posts[0].Scope = repo.ViewPasswordScopeAll
			permissions.posts[0].Membership = test.membership
			permissions.err = nil
			if test.fail {
				permissions.err = errors.New("offline")
			}
			for _, method := range []string{"GET", "HEAD"} {
				req := httptest.NewRequest(method, "/media/"+key, nil)
				req.Header.Set("Sec-Fetch-Dest", "image")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code != test.want {
					t.Fatalf("%s status=%d want=%d", method, rec.Code, test.want)
				}
				if test.want == 200 && rec.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatal("cache allowed")
				}
				req.Header.Set("Range", "bytes=0-1")
				rec = httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if test.want != 200 && rec.Code != test.want {
					t.Fatal("range bypass")
				}
			}
		})
	}
	permissions.err = nil
	permissions.posts = nil
	permissions.public = false
	req := httptest.NewRequest("GET", "/media/"+key, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatal("deleted/unattached object became public")
	}
	req = req.WithContext(context.WithValue(req.Context(), ctxUserID{}, owner))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatal("owner upload preview denied")
	}
}

func TestMediaPublicAliasCannotOverridePrivatePost(t *testing.T) {
	permissions := &testMediaStore{public: true, posts: []repo.MediaPostAccess{{Readable: true}, {Readable: false}}}
	s := &Server{mediaAccess: permissions}
	allowed, err := s.canReadMedia(httptest.NewRequest("GET", "/", nil), "uploads/"+uuid.NewString()+"/a.png")
	if err != nil || allowed {
		t.Fatal("public alias bypassed a private post")
	}
}

type testSessionStore struct {
	active map[uuid.UUID]uuid.UUID
	fail   bool
}

func (m *testSessionStore) CreateAccessSession(_ context.Context, id, user uuid.UUID, _ time.Time) error {
	if m.fail {
		return errors.New("offline")
	}
	m.active[id] = user
	return nil
}
func (m *testSessionStore) AccessSessionActive(_ context.Context, id, user uuid.UUID) (bool, error) {
	if m.fail {
		return false, errors.New("offline")
	}
	return m.active[id] == user, nil
}
func (m *testSessionStore) RevokeAccessSession(_ context.Context, id, user uuid.UUID) error {
	if m.fail {
		return errors.New("offline")
	}
	if m.active[id] == user {
		delete(m.active, id)
	}
	return nil
}

func TestLogoutRevokesSessionAcrossServers(t *testing.T) {
	store := &testSessionStore{active: map[uuid.UUID]uuid.UUID{}}
	s := &Server{secret: []byte("test-secret"), sessions: store}
	other := &Server{secret: s.secret, sessions: store}
	user := uuid.New()
	token, err := s.issueAccessToken(t.Context(), user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.issueAccessToken(t.Context(), user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.handleLogout(rec, req)
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}
	if _, _, ok := other.principalForAccess(t.Context(), token); ok {
		t.Fatal("revoked bearer token accepted")
	}
	if _, _, ok := other.principalForAccess(t.Context(), second); !ok {
		t.Fatal("other session revoked")
	}
	raw, err := authjwt.SignAccess(s.secret, user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := s.principalForAccess(t.Context(), raw); ok {
		t.Fatal("unregistered session accepted")
	}
	store.fail = true
	if _, _, ok := s.principalForAccess(t.Context(), second); ok {
		t.Fatal("store failure bypass")
	}
	req.Header.Set("Authorization", "Bearer "+second)
	rec = httptest.NewRecorder()
	s.handleLogout(rec, req)
	if rec.Code == 200 {
		t.Fatal("failed revoke reported success")
	}
}

func TestLogoutRequiresCSRFFromCookie(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("POST", "/logout", nil)
	r.AddCookie(&http.Cookie{Name: authAccessCookieName, Value: "token"})
	rec := httptest.NewRecorder()
	s.handleLogout(rec, r)
	if rec.Code != 403 {
		t.Fatal("cookie logout lacked CSRF")
	}
}

func TestPushEndpointAndNetworkRestrictions(t *testing.T) {
	for _, raw := range []string{"http://fcm.googleapis.com/x", "https://localhost/x", "https://127.0.0.1/x", "https://[::1]/x", "https://fcm.googleapis.com.evil.example/x", "https://evil@fcm.googleapis.com/x", "https://fcm.googleapis.com:8443/x", "https://fcm.googleapis.com/x#y"} {
		if validPushEndpoint(raw) {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"https://fcm.googleapis.com/x", "https://updates.push.services.mozilla.com/x", "https://web.push.apple.com/x", "https://wns2.notify.windows.com/x"} {
		if !validPushEndpoint(raw) {
			t.Errorf("rejected %s", raw)
		}
	}
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "::ffff:127.0.0.1", "100.64.0.1", "192.0.2.1", "240.0.0.1", "2001:db8::1"} {
		if isPublicOutboundIP(net.ParseIP(raw)) {
			t.Errorf("accepted %s", raw)
		}
	}
	c := newPushHTTPClient()
	if c.Timeout != pushTimeout {
		t.Fatal("missing deadline")
	}
	if c.Transport.(*http.Transport).Proxy != nil {
		t.Fatal("environment proxy enabled")
	}
	if c.CheckRedirect(httptest.NewRequest("GET", "https://example.com", nil), nil) == nil {
		t.Fatal("redirect accepted")
	}
	if validPushKeys("invalid", "invalid") {
		t.Fatal("invalid encryption keys")
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if conn, err := publicOutboundDialContext(ctx, "tcp", "127.0.0.1:1"); err == nil {
		conn.Close()
		t.Fatal("loopback dial allowed")
	}
}
