package httpserver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"glipz.io/backend/internal/authjwt"
	"glipz.io/backend/internal/repo"
)

type testOAuthClients struct {
	row repo.OAuthClientRow
	err error
}

func (c *testOAuthClients) OAuthClientByIDPublic(_ context.Context, id uuid.UUID) (repo.OAuthClientRow, error) {
	if c.err != nil {
		return repo.OAuthClientRow{}, c.err
	}
	if id != c.row.ID {
		return repo.OAuthClientRow{}, repo.ErrNotFound
	}
	return c.row, nil
}

func TestOAuthAccessRequiresExistingClient(t *testing.T) {
	store := &testOAuthClients{row: repo.OAuthClientRow{ID: uuid.New(), UserID: uuid.New()}}
	s := &Server{secret: []byte("test-secret"), oauthClients: store, sessions: &testSessionStore{active: map[uuid.UUID]uuid.UUID{}}}
	user := uuid.New() // Authorization-code subject may differ from app owner.
	for _, subject := range []uuid.UUID{user, store.row.UserID} {
		token, err := authjwt.SignOAuthAccess(s.secret, subject, store.row.ID, "posts:read", time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if got, _, ok := s.principalForAccess(t.Context(), token); !ok || got != subject {
			t.Fatal("valid OAuth principal rejected")
		}
		for _, failure := range []error{repo.ErrNotFound, errors.New("database unavailable")} {
			store.err = failure
			if _, _, ok := s.principalForAccess(t.Context(), token); ok {
				t.Fatal("deleted/unverifiable client accepted")
			}
		}
		store.err = nil
		claims, err := authjwt.Parse(s.secret, token)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"", "invalid", uuid.Nil.String(), uuid.NewString()} {
			claims.ClientID = id
			invalid, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, ok := s.principalForAccess(t.Context(), invalid); ok {
				t.Fatal("invalid client ID accepted")
			}
		}
	}
	token, err := s.issueAccessToken(t.Context(), user, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store.err = errors.New("database unavailable")
	if _, _, ok := s.principalForAccess(t.Context(), token); !ok {
		t.Fatal("OAuth lookup affected ordinary session")
	}
}
