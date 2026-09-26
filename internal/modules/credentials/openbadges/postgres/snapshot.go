package postgres

import (
	"context"
	"errors"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	"github.com/jackc/pgx/v5"
)

// Snapshot reads allocation and Certificate lifecycle from one consistent
// database snapshot. It does not publish or sign a status-list credential.
func (r *Repository) Snapshot(ctx context.Context, listID string) (openbadges.StatusList, []openbadges.StatusFact, error) {
	if r == nil || r.pool == nil || !openbadges.ValidID(listID) {
		return openbadges.StatusList{}, nil, openbadges.ErrInvalidStatusList
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return openbadges.StatusList{}, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var list openbadges.StatusList
	err = tx.QueryRow(ctx, `SELECT id,capacity,status_purpose,lifecycle_revision FROM open_badges.status_list WHERE id=$1`, listID).Scan(&list.ID, &list.Capacity, &list.StatusPurpose, &list.LifecycleRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return list, nil, openbadges.ErrStatusEntryNotFound
	}
	if err != nil {
		return list, nil, err
	}
	rows, err := tx.Query(ctx, `SELECT e.certificate_id,e.status_list_id,e.status_list_index,e.status_purpose,c.status FROM open_badges.status_list_entry e LEFT JOIN credentials.certificate c ON c.id=e.certificate_id WHERE e.status_list_id=$1 ORDER BY e.status_list_index`, listID)
	if err != nil {
		return list, nil, err
	}
	var facts []openbadges.StatusFact
	for rows.Next() {
		var fact openbadges.StatusFact
		var status *string
		if err := rows.Scan(&fact.Entry.CertificateID, &fact.Entry.StatusListID, &fact.Entry.StatusListIndex, &fact.Entry.StatusPurpose, &status); err != nil {
			rows.Close()
			return list, nil, err
		}
		if status != nil {
			fact.CertificatePresent = true
			fact.CertificateStatus = credentials.CertificateStatus(*status)
		}
		facts = append(facts, fact)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return list, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return list, nil, err
	}
	return list, facts, nil
}
