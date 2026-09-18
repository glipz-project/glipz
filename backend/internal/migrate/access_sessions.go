package migrate

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunAccessSessions(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS access_sessions (
 id UUID PRIMARY KEY, user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
 CREATE INDEX IF NOT EXISTS access_sessions_expiry ON access_sessions(expires_at);`)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `CREATE INDEX IF NOT EXISTS posts_object_keys_gin ON posts USING gin(object_keys);`)
	return err
}
