package repo

import (
	"context"

	"github.com/google/uuid"
)

type MediaPostAccess struct {
	ID          uuid.UUID
	Owner       uuid.UUID
	Readable    bool
	HasPassword bool
	Scope       int
	Membership  bool
}

// All references are returned: a public alias must never override a protected post.
func (p *Pool) MediaPostAccess(ctx context.Context, key string, viewer uuid.UUID) ([]MediaPostAccess, error) {
	rows, err := p.db.Query(ctx, `SELECT p.id,p.user_id,
 (u.suspended_at IS NULL AND p.visible_at<=NOW() AND `+postReadableByViewerSQL("p", "$2")+`),
 COALESCE(p.view_password_hash,'')<>'',COALESCE(p.view_password_scope,0),
 COALESCE(btrim(p.membership_provider),'')<>''
 FROM posts p JOIN users u ON u.id=p.user_id WHERE p.object_keys @> ARRAY[$1]::text[]`, key, viewer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MediaPostAccess{}
	for rows.Next() {
		var a MediaPostAccess
		if err := rows.Scan(&a.ID, &a.Owner, &a.Readable, &a.HasPassword, &a.Scope, &a.Membership); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (p *Pool) PublicMediaAsset(ctx context.Context, key string) (bool, error) {
	var ok bool
	err := p.db.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM users WHERE suspended_at IS NULL AND (avatar_object_key=$1 OR header_object_key=$1))
 OR EXISTS(SELECT 1 FROM communities WHERE icon_object_key=$1 OR header_object_key=$1)
 OR EXISTS(SELECT 1 FROM custom_emojis WHERE object_key=$1 AND is_enabled=true)`, key).Scan(&ok)
	return ok, err
}

func (p *Pool) DMObjectReadable(ctx context.Context, key string, viewer uuid.UUID) (bool, error) {
	var ok bool
	err := p.db.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM dm_messages m JOIN dm_threads t ON t.id=m.thread_id
 WHERE (t.user_low_id=$2 OR t.user_high_id=$2)
 AND m.attachments @> jsonb_build_array(jsonb_build_object('object_key',$1::text)))`, key, viewer).Scan(&ok)
	return ok, err
}

func (p *Pool) RemoteDMObjectReadable(ctx context.Context, key, acct, localHost string) (bool, error) {
	var ok bool
	err := p.db.QueryRow(ctx, `SELECT EXISTS(
 SELECT 1 FROM federation_dm_messages m JOIN federation_dm_threads t ON t.thread_id=m.thread_id
 JOIN users u ON u.id=t.local_user_id
 WHERE t.remote_acct=$2 AND t.state='accepted' AND u.suspended_at IS NULL
 AND m.sender_acct=u.handle||'@'||$3 AND $1 LIKE 'uploads/'||u.id::text||'/%'
 AND m.attachments @> jsonb_build_array(jsonb_build_object('object_key',$1::text)))`, key, acct, localHost).Scan(&ok)
	return ok, err
}
