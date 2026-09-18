package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"glipz.io/backend/internal/repo"
)

type mediaAccessStore interface {
	MediaPostAccess(context.Context, string, uuid.UUID) ([]repo.MediaPostAccess, error)
	PublicMediaAsset(context.Context, string) (bool, error)
	DMObjectReadable(context.Context, string, uuid.UUID) (bool, error)
}

func (s *Server) mediaStore() mediaAccessStore {
	if s.mediaAccess != nil {
		return s.mediaAccess
	}
	if s.db != nil {
		return s.db
	}
	return nil
}

func (s *Server) canReadMedia(r *http.Request, key string) (bool, error) {
	store := s.mediaStore()
	if store == nil {
		return false, errors.New("media authorization unavailable")
	}
	viewer, _ := userIDFrom(r.Context())
	remoteViewer := s.remoteMediaViewer(r)
	posts, err := store.MediaPostAccess(r.Context(), key, viewer)
	if err != nil {
		return false, err
	}
	if len(posts) > 0 {
		// Intersect permissions for shared objects, avoiding a public-reference bypass.
		for _, p := range posts {
			if !p.Readable {
				return false, nil
			}
			if viewer == p.Owner {
				continue
			}
			if p.Membership || (p.HasPassword && scopeProtectsMedia(repo.EffectiveViewPasswordScope(true, p.Scope))) {
				if !s.localPostUnlocked(r.Context(), viewer, p.ID) && !s.remotePostUnlocked(r.Context(), remoteViewer, p.ID) {
					return false, nil
				}
			}
		}
		return true, nil
	}
	public, err := store.PublicMediaAsset(r.Context(), key)
	if err != nil || public {
		return public, err
	}
	if viewer == uuid.Nil {
		if remoteViewer != "" && s.db != nil {
			return s.db.RemoteDMObjectReadable(r.Context(), key, remoteViewer, s.federationDisplayHost())
		}
		return false, nil
	}
	if strings.HasPrefix(key, "uploads/"+viewer.String()+"/") {
		return true, nil
	}
	return store.DMObjectReadable(r.Context(), key, viewer)
}
