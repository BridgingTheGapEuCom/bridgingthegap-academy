package postgres

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IndexCandidate returns a uniformly selected index in [0, capacity).
// Production uses crypto/rand; injection permits deterministic collision tests.
type IndexCandidate func(capacity int) (int, error)

type Repository struct {
	pool      *pgxpool.Pool
	capacity  int
	candidate IndexCandidate
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, capacity: openbadges.DefaultStatusListCapacity, candidate: secureCandidate}
}
func (r *Repository) WithCapacity(capacity int) *Repository {
	copy := *r
	copy.capacity = capacity
	return &copy
}
func (r *Repository) WithIndexCandidate(candidate IndexCandidate) *Repository {
	copy := *r
	copy.candidate = candidate
	return &copy
}
func secureCandidate(capacity int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(capacity)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func (r *Repository) EnsureEntry(ctx context.Context, certificateID string) (openbadges.StatusListEntry, error) {
	if r == nil || r.pool == nil || r.capacity < openbadges.DefaultStatusListCapacity || r.candidate == nil {
		return openbadges.StatusListEntry{}, openbadges.ErrInvalidConfiguration
	}
	if !openbadges.ValidID(certificateID) {
		return openbadges.StatusListEntry{}, openbadges.ErrInvalidCertificate
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return openbadges.StatusListEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// One transaction-level lock serializes v1 revocation allocations across processes,
	// including list rollover and concurrent requests for the same Certificate.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1987203101)`); err != nil {
		return openbadges.StatusListEntry{}, err
	}
	var e openbadges.StatusListEntry
	err = tx.QueryRow(ctx, `SELECT certificate_id,status_list_id,status_list_index,status_purpose FROM open_badges.status_list_entry WHERE certificate_id=$1`, certificateID).Scan(&e.CertificateID, &e.StatusListID, &e.StatusListIndex, &e.StatusPurpose)
	if err == nil {
		return e, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return e, err
	}
	var list string
	var capacity int
	err = tx.QueryRow(ctx, `SELECT id,capacity FROM open_badges.status_list WHERE status_purpose='revocation' AND next_index < capacity ORDER BY created_at,id LIMIT 1 FOR UPDATE`).Scan(&list, &capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `INSERT INTO open_badges.status_list(status_purpose,capacity) VALUES('revocation',$1) RETURNING id,capacity`, r.capacity).Scan(&list, &capacity)
	}
	if err != nil {
		return e, err
	}
	// Collisions are cheap at low occupancy. Near capacity, a bounded random probe
	// falls back to the first free slot following a random starting position.
	for attempt := 0; attempt < 16; attempt++ {
		candidate, err := r.candidate(capacity)
		if err != nil {
			return e, fmt.Errorf("status index randomness: %w", err)
		}
		if candidate < 0 || candidate >= capacity {
			return e, openbadges.ErrInvalidStatusList
		}
		inserted, err := insertEntry(ctx, tx, certificateID, list, candidate)
		if err != nil {
			return e, err
		}
		if inserted {
			e = openbadges.StatusListEntry{CertificateID: certificateID, StatusListID: list, StatusListIndex: candidate, StatusPurpose: openbadges.RevocationPurpose}
			break
		}
		if attempt == 15 {
			err = tx.QueryRow(ctx, `SELECT candidate FROM generate_series(0,$2::integer - 1) AS candidate WHERE NOT EXISTS (SELECT 1 FROM open_badges.status_list_entry WHERE status_list_id=$1 AND status_list_index=candidate) ORDER BY ((candidate - $3::integer + $2::integer) % $2::integer) LIMIT 1`, list, capacity, candidate).Scan(&candidate)
			if errors.Is(err, pgx.ErrNoRows) {
				return e, openbadges.ErrStatusListFull
			}
			if err != nil {
				return e, err
			}
			inserted, err = insertEntry(ctx, tx, certificateID, list, candidate)
			if err != nil {
				return e, err
			}
			if !inserted {
				return e, openbadges.ErrStatusListFull
			}
			e = openbadges.StatusListEntry{CertificateID: certificateID, StatusListID: list, StatusListIndex: candidate, StatusPurpose: openbadges.RevocationPurpose}
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE open_badges.status_list SET next_index=next_index+1 WHERE id=$1 AND next_index<capacity`, list); err != nil {
		return openbadges.StatusListEntry{}, err
	}
	return e, tx.Commit(ctx)
}

func insertEntry(ctx context.Context, tx pgx.Tx, certificateID, list string, index int) (bool, error) {
	var inserted string
	err := tx.QueryRow(ctx, `INSERT INTO open_badges.status_list_entry(certificate_id,status_list_id,status_list_index,status_purpose) VALUES($1,$2,$3,'revocation') ON CONFLICT (status_list_id,status_list_index) DO NOTHING RETURNING certificate_id`, certificateID, list, index).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
