package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
)

func (p *Pool) CreateAccessSession(ctx context.Context, id, user uuid.UUID, expires time.Time) error {
	// Bounded cleanup keeps expired sessions from accumulating without a separate job.
	if _, err := p.db.Exec(ctx, `DELETE FROM access_sessions WHERE id IN (SELECT id FROM access_sessions WHERE expires_at<=NOW() LIMIT 100)`); err != nil {
		return err
	}
	_, err := p.db.Exec(ctx, `INSERT INTO access_sessions(id,user_id,expires_at) VALUES($1,$2,$3)`, id, user, expires)
	return err
}
func (p *Pool) AccessSessionActive(ctx context.Context, id, user uuid.UUID) (bool, error) {
	var active bool
	err := p.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM access_sessions WHERE id=$1 AND user_id=$2 AND expires_at>NOW())`, id, user).Scan(&active)
	return active, err
}
func (p *Pool) RevokeAccessSession(ctx context.Context, id, user uuid.UUID) error {
	_, err := p.db.Exec(ctx, `DELETE FROM access_sessions WHERE id=$1 AND user_id=$2`, id, user)
	return err
}
