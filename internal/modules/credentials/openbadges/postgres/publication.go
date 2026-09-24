package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/signing"
	"github.com/jackc/pgx/v5"
)

type PublishedSnapshot struct {
	Document          []byte
	EncodedList       string
	KeyID             string
	GeneratedAt       time.Time
	LifecycleRevision int64
}

func (r *Repository) RegisterMethod(ctx context.Context, key signing.Multikey) error {
	if r == nil || r.pool == nil {
		return openbadges.ErrInvalidConfiguration
	}
	tag, err := r.pool.Exec(ctx, `INSERT INTO open_badges.verification_method(id,controller,public_key_multibase) VALUES($1,$2,$3) ON CONFLICT (id) DO UPDATE SET id=EXCLUDED.id WHERE open_badges.verification_method.controller=EXCLUDED.controller AND open_badges.verification_method.public_key_multibase=EXCLUDED.public_key_multibase`, key.ID, key.Controller, key.PublicKeyMultibase)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return signing.ErrInvalidKey
	}
	return nil
}
func (r *Repository) Methods(ctx context.Context, controller string) ([]signing.Multikey, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,controller,public_key_multibase FROM open_badges.verification_method WHERE controller=$1 ORDER BY created_at,id`, controller)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []signing.Multikey
	for rows.Next() {
		var m signing.Multikey
		m.Type = "Multikey"
		if err = rows.Scan(&m.ID, &m.Controller, &m.PublicKeyMultibase); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (r *Repository) GetSignedCredential(ctx context.Context, certificateID string) ([]byte, error) {
	var doc []byte
	err := r.pool.QueryRow(ctx, `SELECT secured_document FROM open_badges.signed_credential WHERE certificate_id=$1`, certificateID).Scan(&doc)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, openbadges.ErrStatusEntryNotFound
	}
	return doc, err
}
func (r *Repository) SaveSignedCredential(ctx context.Context, certificateID, keyID string, createdAt time.Time, document []byte) ([]byte, error) {
	_, err := r.pool.Exec(ctx, `INSERT INTO open_badges.signed_credential(certificate_id,key_id,created_at,secured_document) VALUES($1,$2,$3,$4) ON CONFLICT (certificate_id) DO NOTHING`, certificateID, keyID, createdAt, document)
	if err != nil {
		return nil, err
	}
	return r.GetSignedCredential(ctx, certificateID)
}
func (r *Repository) GetPublishedSnapshot(ctx context.Context, listID string) (PublishedSnapshot, error) {
	var snap PublishedSnapshot
	err := r.pool.QueryRow(ctx, `SELECT secured_document,encoded_list,key_id,generated_at,lifecycle_revision FROM open_badges.published_status_list WHERE status_list_id=$1`, listID).Scan(&snap.Document, &snap.EncodedList, &snap.KeyID, &snap.GeneratedAt, &snap.LifecycleRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, openbadges.ErrStatusEntryNotFound
	}
	return snap, err
}

// PublishCurrentSnapshot holds an exclusive list row lock while reading
// lifecycle state, signing, and atomically replacing the complete snapshot.
// It serializes concurrent publishers and revocation revision triggers.
func (r *Repository) PublishCurrentSnapshot(ctx context.Context, listID string, build func(openbadges.StatusList, []openbadges.StatusFact) (PublishedSnapshot, error)) (PublishedSnapshot, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return PublishedSnapshot{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var list openbadges.StatusList
	err = tx.QueryRow(ctx, `SELECT id,capacity,status_purpose,lifecycle_revision FROM open_badges.status_list WHERE id=$1 FOR UPDATE`, listID).Scan(&list.ID, &list.Capacity, &list.StatusPurpose, &list.LifecycleRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return PublishedSnapshot{}, openbadges.ErrStatusEntryNotFound
	}
	if err != nil {
		return PublishedSnapshot{}, err
	}
	rows, err := tx.Query(ctx, `SELECT e.certificate_id,e.status_list_id,e.status_list_index,e.status_purpose,c.status FROM open_badges.status_list_entry e LEFT JOIN credentials.certificate c ON c.id=e.certificate_id WHERE e.status_list_id=$1 ORDER BY e.status_list_index`, listID)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	var facts []openbadges.StatusFact
	for rows.Next() {
		var fact openbadges.StatusFact
		var status *string
		if err = rows.Scan(&fact.Entry.CertificateID, &fact.Entry.StatusListID, &fact.Entry.StatusListIndex, &fact.Entry.StatusPurpose, &status); err != nil {
			rows.Close()
			return PublishedSnapshot{}, err
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
		return PublishedSnapshot{}, err
	}
	snap, err := build(list, facts)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	if snap.LifecycleRevision != list.LifecycleRevision {
		return PublishedSnapshot{}, openbadges.ErrInvalidStatusList
	}
	_, err = tx.Exec(ctx, `INSERT INTO open_badges.published_status_list(status_list_id,key_id,secured_document,encoded_list,generated_at,lifecycle_revision) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (status_list_id) DO UPDATE SET key_id=EXCLUDED.key_id,secured_document=EXCLUDED.secured_document,encoded_list=EXCLUDED.encoded_list,generated_at=EXCLUDED.generated_at,lifecycle_revision=EXCLUDED.lifecycle_revision`, listID, snap.KeyID, snap.Document, snap.EncodedList, snap.GeneratedAt, snap.LifecycleRevision)
	if err != nil {
		return PublishedSnapshot{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PublishedSnapshot{}, err
	}
	return snap, nil
}

// CurrentSnapshot returns a published snapshot only when it matches durable
// Certificate lifecycle revision. A failed refresh therefore cannot present a
// stale ACTIVE bit as current after revocation.
func (r *Repository) CurrentSnapshot(ctx context.Context, listID string) (PublishedSnapshot, error) {
	var snap PublishedSnapshot
	err := r.pool.QueryRow(ctx, `SELECT p.secured_document,p.encoded_list,p.key_id,p.generated_at,p.lifecycle_revision FROM open_badges.published_status_list p JOIN open_badges.status_list l ON l.id=p.status_list_id WHERE p.status_list_id=$1 AND p.lifecycle_revision=l.lifecycle_revision`, listID).Scan(&snap.Document, &snap.EncodedList, &snap.KeyID, &snap.GeneratedAt, &snap.LifecycleRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, openbadges.ErrStatusEntryNotFound
	}
	return snap, err
}
