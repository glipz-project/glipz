package httpserver

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"glipz.io/backend/internal/repo"
)

// Called only after cryptographic entitlement verification; never extends its expiry.
func unlockGrantTTL(entitlement string, membership bool) time.Duration {
	if !membership {
		return postUnlockRedisTTL
	}
	claims := &jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(entitlement, claims); err != nil || claims.ExpiresAt == nil {
		return 0
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 15*time.Minute {
		return 15 * time.Minute
	}
	return ttl
}

func postLockVersion(row repo.PostSensitive) string {
	hash := ""
	if row.ViewPasswordHash != nil {
		hash = *row.ViewPasswordHash
	}
	b, _ := json.Marshal([]any{hash, row.ViewPasswordScope, row.MembershipProvider, row.MembershipCreatorID, row.MembershipTierID})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func remoteMediaGrantKey(acct string, post uuid.UUID) string {
	sum := sha256.Sum256([]byte(strings.ToLower(acct)))
	return "media:remote:v1:" + post.String() + ":" + hex.EncodeToString(sum[:])
}

func (s *Server) localPostUnlocked(ctx context.Context, viewer, post uuid.UUID) bool {
	if viewer == uuid.Nil || s.rdb == nil || s.db == nil {
		return false
	}
	value, err := s.rdb.Get(ctx, postUnlockRedisKey(viewer, post)).Result()
	if err != nil {
		return false
	}
	row, err := s.db.PostSensitiveByID(ctx, post)
	return err == nil && value == postLockVersion(row)
}

// The viewer is part of the signed request target, never trusted from a header alone.
func (s *Server) remoteMediaViewer(r *http.Request) string {
	acct := r.URL.Query().Get("glipz_viewer")
	if acct == "" {
		return ""
	}
	_, host, err := splitAcct(acct)
	if err != nil {
		return ""
	}
	verified, err := s.verifyFederationRequest(r, nil)
	if err != nil || !strings.EqualFold(host, verified.InstanceHost) {
		return ""
	}
	return acct
}

func (s *Server) remotePostUnlocked(ctx context.Context, acct string, post uuid.UUID) bool {
	if acct == "" || s.rdb == nil || s.db == nil {
		return false
	}
	value, err := s.rdb.Get(ctx, remoteMediaGrantKey(acct, post)).Result()
	if err != nil {
		return false
	}
	row, err := s.db.PostSensitiveByID(ctx, post)
	return err == nil && value == postLockVersion(row)
}

func (s *Server) signMediaRequest(req *http.Request) {
	_, priv := s.federationServerKeys()
	ts := time.Now().UTC().Format(time.RFC3339)
	nonce := uuid.NewString()
	sig := ed25519.Sign(priv, federationSignatureMessage(req.Method, federationSignedRequestTarget(req.URL), ts, nonce, nil, 2))
	req.Header.Set("X-Glipz-Instance", s.federationDisplayHost())
	req.Header.Set("X-Glipz-Key-Id", s.federationServerKeyID())
	req.Header.Set("X-Glipz-Protocol-Version", federationProtocolVersion)
	req.Header.Set("X-Glipz-Timestamp", ts)
	req.Header.Set("X-Glipz-Nonce", nonce)
	req.Header.Set("X-Glipz-Signature", base64.StdEncoding.EncodeToString(sig))
}

// Do not turn the public proxy into a signing oracle for another user's grants.
func (s *Server) signViewerMediaRequest(w http.ResponseWriter, r, req *http.Request) bool {
	acct := req.URL.Query().Get("glipz_viewer")
	if acct == "" {
		return true
	}
	uid, ok := userIDFrom(r.Context())
	if !ok || s.db == nil {
		http.NotFound(w, r)
		return false
	}
	user, err := s.db.UserByID(r.Context(), uid)
	if err != nil || !strings.EqualFold(acct, s.localFullAcct(user.Handle)) || !strings.HasPrefix(req.URL.Path, "/api/v1/media/object/") {
		http.NotFound(w, r)
		return false
	}
	s.signMediaRequest(req)
	return true
}

func mediaURLForRemoteViewer(raw, acct string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	q := u.Query()
	q.Set("glipz_viewer", acct)
	u.RawQuery = q.Encode()
	return u.String()
}
