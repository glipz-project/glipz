package migrate

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func tableExists(ctx context.Context, pool *pgxpool.Pool, tableName string) (bool, error) {
	var n int
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1
	`, tableName).Scan(&n)
	return n > 0, err
}
