package repo

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// The stored actor is authoritative, including after a verified account move.
// Never derive ownership from portable IDs or remote-account metadata supplied
// by the event being authorized.
func lockFederatedPostOwner(ctx context.Context, tx pgx.Tx, objectIRI, actorIRI string) (bool, error) {
	var owner string
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT actor_iri, deleted_at IS NOT NULL
		FROM federation_incoming_posts WHERE object_iri = $1 FOR UPDATE`, strings.TrimSpace(objectIRI)).Scan(&owner, &deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	owner, actorIRI = strings.TrimSpace(owner), strings.TrimSpace(actorIRI)
	if !strings.Contains(owner, "://") && !strings.Contains(actorIRI, "://") {
		owner, actorIRI = NormalizeFederationTargetAcct(owner), NormalizeFederationTargetAcct(actorIRI)
	}
	if actorIRI == "" || owner != actorIRI {
		return false, ErrForbidden
	}
	return !deleted, nil
}

// Like totals are relayed by the post's instance; the envelope actor is the
// person who liked the post and need not be its owner.
func (p *Pool) SetFederatedIncomingLikeCountFromInstance(ctx context.Context, objectIRI, instanceHost string, count int64) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owner string
	err = tx.QueryRow(ctx, `SELECT actor_iri FROM federation_incoming_posts WHERE object_iri = $1 AND deleted_at IS NULL FOR UPDATE`, objectIRI).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	parts := strings.Split(owner, "@")
	if len(parts) != 2 || strings.TrimPrefix(strings.ToLower(parts[1]), "www.") != strings.TrimPrefix(strings.ToLower(instanceHost), "www.") {
		return ErrForbidden
	}
	if _, err := tx.Exec(ctx, `UPDATE federation_incoming_posts SET like_count = $2 WHERE object_iri = $1`, objectIRI, maxInt64(count, 0)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ApplyFederatedPostEvent authorizes and applies an owner-only event atomically.
// Concurrent creates are serialized by the unique object_iri constraint; an
// existing row is locked and checked before any update, including poll tallies.
func (p *Pool) ApplyFederatedPostEvent(ctx context.Context, kind string, in InsertFederatedIncomingInput, poll *FederatedIncomingPollSnapshot) (bool, error) {
	if strings.TrimSpace(in.ObjectIRI) == "" || strings.TrimSpace(in.ActorIRI) == "" {
		return false, ErrForbidden
	}
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	inserted := false
	switch kind {
	case "post_created", "repost_created":
		inserted, err = insertFederatedIncomingPost(ctx, tx, in)
		if err != nil {
			return false, err
		}
	case "post_updated", "post_deleted", "poll_tally_updated":
	default:
		return false, ErrForbidden
	}
	exists, err := lockFederatedPostOwner(ctx, tx, in.ObjectIRI, in.ActorIRI)
	if err != nil || !exists {
		return false, err
	}
	if in.EventRemoteAccount != nil {
		metadata := *in.EventRemoteAccount
		if NormalizeFederationTargetAcct(metadata.CurrentAcct) != NormalizeFederationTargetAcct(in.ActorIRI) {
			return false, ErrForbidden
		}
		account, err := upsertRemoteAccount(ctx, tx, metadata, in.ActorIRI)
		if err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `UPDATE federation_incoming_posts SET remote_account_id = $2 WHERE object_iri = $1`, strings.TrimSpace(in.ObjectIRI), account.ID); err != nil {
			return false, err
		}
	}
	if kind == "post_deleted" {
		_, err = tx.Exec(ctx, `UPDATE federation_incoming_posts SET deleted_at = NOW() WHERE object_iri = $1`, strings.TrimSpace(in.ObjectIRI))
	} else {
		if kind == "poll_tally_updated" {
			_, err = tx.Exec(ctx, `UPDATE federation_incoming_posts SET like_count = $2 WHERE object_iri = $1`, strings.TrimSpace(in.ObjectIRI), maxInt64(in.LikeCount, 0))
		} else if !inserted {
			err = updateFederatedIncomingFromNote(ctx, tx, in.ObjectIRI, in.CaptionText, in.MediaType, in.MediaURLs, in.IsNSFW, in.PublishedAt, in.LikeCount, in.ReplyToObjectIRI, in.RepostOfObjectIRI, in.RepostComment, in.HasViewPassword, in.ViewPasswordScope, in.ViewPasswordTextRanges, in.UnlockURL, in.MembershipProvider, in.MembershipCreatorID, in.MembershipTierID)
		}
		if err == nil {
			err = syncFederatedIncomingPoll(ctx, tx, in.ObjectIRI, poll)
		}
		if err == nil && kind != "poll_tally_updated" {
			_, err = tx.Exec(ctx, `UPDATE federation_incoming_posts SET actor_name = $2,
				actor_icon_url = NULLIF($3, ''), actor_profile_url = NULLIF($4, ''), actor_acct = COALESCE(NULLIF($5, ''), actor_acct) WHERE object_iri = $1`,
				strings.TrimSpace(in.ObjectIRI), truncateRunes(strings.TrimSpace(in.ActorName), 200), strings.TrimSpace(in.ActorIconURL), strings.TrimSpace(in.ActorProfileURL), strings.TrimSpace(in.ActorAcct))
		}
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
