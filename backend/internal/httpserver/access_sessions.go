package httpserver

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"glipz.io/backend/internal/authjwt"
)

type accessSessionStore interface {
	CreateAccessSession(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	AccessSessionActive(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	RevokeAccessSession(context.Context, uuid.UUID, uuid.UUID) error
}

func (s *Server) sessionStore() accessSessionStore {
	if s.sessions != nil {
		return s.sessions
	}
	if s.db != nil {
		return s.db
	}
	return nil
}

func (s *Server) issueAccessToken(ctx context.Context, user uuid.UUID, ttl time.Duration) (string, error) {
	store := s.sessionStore()
	if store == nil {
		return "", errors.New("session store unavailable")
	}
	token, err := authjwt.SignAccess(s.secret, user, ttl)
	if err != nil {
		return "", err
	}
	claims, err := authjwt.Parse(s.secret, token)
	if err != nil {
		return "", err
	}
	id, err := uuid.Parse(claims.ID)
	if err != nil {
		return "", err
	}
	if err = store.CreateAccessSession(ctx, id, user, claims.ExpiresAt.Time); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Server) userSessionActive(ctx context.Context, claims *authjwt.Claims, user uuid.UUID) bool {
	id, err := uuid.Parse(claims.ID)
	if err != nil || claims.ExpiresAt == nil || s.sessionStore() == nil {
		return false
	}
	active, err := s.sessionStore().AccessSessionActive(ctx, id, user)
	return err == nil && active
}
