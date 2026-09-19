package httpserver

import (
	"context"

	"github.com/google/uuid"

	"glipz.io/backend/internal/authjwt"
	"glipz.io/backend/internal/repo"
)

type oauthClientStore interface {
	OAuthClientByIDPublic(context.Context, uuid.UUID) (repo.OAuthClientRow, error)
}

func (s *Server) oauthClientActive(ctx context.Context, claims *authjwt.Claims) bool {
	id, err := uuid.Parse(claims.ClientID)
	if err != nil || id == uuid.Nil {
		return false
	}
	store := s.oauthClients
	if store == nil && s.db != nil {
		store = s.db
	}
	if store == nil {
		return false
	}
	_, err = store.OAuthClientByIDPublic(ctx, id)
	return err == nil
}
