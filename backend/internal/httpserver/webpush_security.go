package httpserver

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"errors"
	"expvar"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"glipz.io/backend/internal/repo"
)

const pushTimeout = 15 * time.Second
const pushWorkers = 4
const pushQueueSize = 128

var pushSecurityMetrics = expvar.NewMap("web_push_security")
var pushHTTPClient = newPushHTTPClient()

type pushSubscriptionStore interface {
	ListPushSubscriptionsByUser(context.Context, uuid.UUID) ([]repo.PushSubscription, error)
	DeletePushSubscription(context.Context, uuid.UUID, string) error
	DeletePushSubscriptionByEndpoint(context.Context, string) error
	MarkPushSubscriptionFailure(context.Context, string, string) error
	MarkPushSubscriptionSuccess(context.Context, string) error
}

func (s *Server) pushStore() pushSubscriptionStore {
	if s.pushSubscriptions != nil {
		return s.pushSubscriptions
	}
	if s.db != nil {
		return s.db
	}
	return nil
}

// Only browser-operated push services are accepted. Subdomains use label boundaries.
func validPushEndpoint(raw string) bool {
	if len(raw) > 4096 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" ||
		host == "web.push.apple.com" || strings.HasSuffix(host, ".notify.windows.com")
}

func validPushKeys(public, auth string) bool {
	decode := func(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "=")) }
	p, err := decode(public)
	if err != nil {
		return false
	}
	if _, err = ecdh.P256().NewPublicKey(p); err != nil {
		return false
	}
	a, err := decode(auth)
	return err == nil && len(a) == 16
}

func newPushHTTPClient() *http.Client {
	c := newPublicOutboundHTTPClient(pushTimeout)
	c.Transport.(*http.Transport).Proxy = nil
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return errors.New("push redirects disabled") }
	return c
}

type pushJob struct {
	user    uuid.UUID
	payload webPushNotification
	queued  time.Time
}
type pushQueue struct {
	once sync.Once
	jobs chan pushJob
}

func (s *Server) enqueuePush(user uuid.UUID, payload webPushNotification) {
	s.push.once.Do(func() {
		s.push.jobs = make(chan pushJob, pushQueueSize)
		for i := 0; i < pushWorkers; i++ {
			go func() {
				for job := range s.push.jobs {
					if time.Since(job.queued) > time.Minute {
						pushSecurityMetrics.Add("expired", 1)
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
					s.sendWebPushToUser(ctx, job.user, job.payload)
					cancel()
				}
			}()
		}
	})
	select {
	case s.push.jobs <- pushJob{user, payload, time.Now()}:
		pushSecurityMetrics.Add("queued", 1)
	default:
		pushSecurityMetrics.Add("dropped", 1)
	}
}
