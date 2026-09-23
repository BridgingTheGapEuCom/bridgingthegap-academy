package postgres

import (
	"context"
	"errors"
	"fmt"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool     *pgxpool.Pool
	capacity int
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, capacity: openbadges.DefaultStatusListCapacity}
}
func (r *Repository) WithCapacity(capacity int) *Repository {
	copy := *r
	copy.capacity = capacity
	return &copy
}
func (r *Repository) EnsureEntry(ctx context.Context, certificateID string) (openbadges.StatusListEntry, error) {
	if r == nil || r.pool == nil || r.capacity < openbadges.DefaultStatusListCapacity {
		return openbadges.StatusListEntry{}, openbadges.ErrInvalidConfiguration
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return openbadges.StatusListEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var e openbadges.StatusListEntry
	err = tx.QueryRow(ctx, `SELECT certificate_id,status_list_id,status_list_index,status_purpose FROM open_badges.status_list_entry WHERE certificate_id=$1`, certificateID).Scan(&e.CertificateID, &e.StatusListID, &e.StatusListIndex, &e.StatusPurpose)
	if err == nil {
		return e, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return e, err
	}
	var list string
	var index int
	err = tx.QueryRow(ctx, `SELECT id,next_index FROM open_badges.status_list WHERE status_purpose='revocation' AND next_index < capacity ORDER BY created_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&list, &index)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO open_badges.status_list(status_purpose,capacity) VALUES('revocation',$1) RETURNING id,next_index`, r.capacity).Scan(&list, &index)
	}
	if err != nil {
		return e, err
	}
	if _, err = tx.Exec(ctx, `UPDATE open_badges.status_list SET next_index=next_index+1 WHERE id=$1`, list); err != nil {
		return e, err
	}
	e = openbadges.StatusListEntry{CertificateID: certificateID, StatusListID: list, StatusListIndex: index, StatusPurpose: openbadges.RevocationPurpose}
	if _, err = tx.Exec(ctx, `INSERT INTO open_badges.status_list_entry(certificate_id,status_list_id,status_list_index,status_purpose) VALUES($1,$2,$3,$4)`, e.CertificateID, e.StatusListID, e.StatusListIndex, e.StatusPurpose); err != nil {
		if get := tx.QueryRow(ctx, `SELECT certificate_id,status_list_id,status_list_index,status_purpose FROM open_badges.status_list_entry WHERE certificate_id=$1`, certificateID).Scan(&e.CertificateID, &e.StatusListID, &e.StatusListIndex, &e.StatusPurpose); get == nil {
			return e, tx.Commit(ctx)
		}
		return openbadges.StatusListEntry{}, fmt.Errorf("allocate status entry: %w", err)
	}
	return e, tx.Commit(ctx)
}
