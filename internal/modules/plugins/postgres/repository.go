package postgres

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/plugins/db/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var _ plugins.Repository = (*Repository)(nil)

func New(db *pgxpool.Pool) *Repository { return &Repository{pool: db, q: sqlc.New(db)} }

func (r *Repository) RegisterKey(ctx context.Context, k plugins.VerificationKey) error {
	if err := k.Validate(); err != nil {
		return err
	}
	ids := make([]string, len(k.AllowedPluginIDs))
	for i, id := range k.AllowedPluginIDs {
		ids[i] = string(id)
	}
	_, err := r.q.RegisterVerificationKey(ctx, sqlc.RegisterVerificationKeyParams{KeyID: k.ID, PublicKey: []byte(k.PublicKey), Purpose: string(k.Purpose), AllowedPluginIds: ids, Enabled: k.Enabled})
	if errors.Is(err, pgx.ErrNoRows) {
		existing, getErr := r.GetKey(ctx, k.ID)
		if getErr == nil && sameKeyIdentity(existing, k) {
			return nil
		}
		return plugins.ErrKeyIDConflict
	}
	return storageError(err)
}
func (r *Repository) GetKey(ctx context.Context, id string) (plugins.VerificationKey, error) {
	row, err := r.q.GetVerificationKey(ctx, id)
	if err != nil {
		return plugins.VerificationKey{}, storageError(err)
	}
	allowed := make([]plugins.PluginID, len(row.AllowedPluginIds))
	for i, v := range row.AllowedPluginIds {
		allowed[i] = plugins.PluginID(v)
	}
	k := plugins.VerificationKey{ID: row.KeyID, PublicKey: ed25519.PublicKey(append([]byte(nil), row.PublicKey...)), Purpose: plugins.KeyPurpose(row.Purpose), AllowedPluginIDs: allowed, Enabled: row.Enabled}
	if err := k.Validate(); err != nil {
		return plugins.VerificationKey{}, plugins.ErrStorage
	}
	return k, nil
}
func (r *Repository) SetKeyEnabled(ctx context.Context, id string, enabled bool) error {
	n, err := r.q.SetVerificationKeyEnabled(ctx, sqlc.SetVerificationKeyEnabledParams{KeyID: id, Enabled: enabled})
	if err != nil {
		return storageError(err)
	}
	if n == 0 {
		return plugins.ErrNotFound
	}
	return nil
}
func (r *Repository) RegisterRelease(ctx context.Context, in plugins.InstalledRelease, resources []plugins.InstalledResource) (plugins.InstalledRelease, bool, error) {
	if in.Release.Validate() != nil {
		return plugins.InstalledRelease{}, false, plugins.ErrInvalidRelease
	}
	id, err := uuid(in.InstallationID)
	if err != nil {
		return plugins.InstalledRelease{}, false, plugins.ErrInvalidRelease
	}
	manifest, err := json.Marshal(in.Manifest)
	if err != nil {
		return plugins.InstalledRelease{}, false, plugins.ErrInvalidRelease
	}
	var key, value pgtype.Text
	var payload []byte
	if in.Signature != nil {
		key = pgtype.Text{String: in.Signature.KeyID, Valid: true}
		value = pgtype.Text{String: in.Signature.Value, Valid: true}
		payload = append([]byte(nil), in.Signature.SigningPayload...)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return plugins.InstalledRelease{}, false, plugins.ErrStorage
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	row, err := q.RegisterInstalledRelease(ctx, sqlc.RegisterInstalledReleaseParams{InstallationID: id, PluginID: string(in.Release.PluginID), Version: in.Release.Version, ArtifactDigest: in.Release.ArtifactDigest, Manifest: manifest, SignatureKeyID: key, SignatureValue: value, SigningPayload: payload, RegisteredTrust: string(in.RegisteredTrust), InstalledAt: pgtype.Timestamptz{Time: in.InstalledAt, Valid: true}})
	created := true
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = q.GetInstalledRelease(ctx, sqlc.GetInstalledReleaseParams{PluginID: string(in.Release.PluginID), Version: in.Release.Version})
		if err != nil {
			return plugins.InstalledRelease{}, false, storageError(err)
		}
		existing, getErr := mapRelease(row)
		if getErr != nil {
			return plugins.InstalledRelease{}, false, getErr
		}
		if existing.Release.ArtifactDigest != in.Release.ArtifactDigest {
			return plugins.InstalledRelease{}, false, plugins.ErrReleaseConflict
		}
		id, err = uuid(existing.InstallationID)
		if err != nil {
			return plugins.InstalledRelease{}, false, plugins.ErrStorage
		}
		created = false
	}
	if err != nil {
		return plugins.InstalledRelease{}, false, storageError(err)
	}
	for _, resource := range resources {
		if int64(len(resource.Content)) != resource.Size || digest(resource.Content) != resource.SHA256 {
			return plugins.InstalledRelease{}, false, plugins.ErrResourceInvalid
		}
		if err := q.RegisterInstalledResource(ctx, sqlc.RegisterInstalledResourceParams{InstallationID: id, ResourcePath: resource.Path, Sha256Digest: resource.SHA256, ByteSize: resource.Size, Content: append([]byte(nil), resource.Content...)}); err != nil {
			return plugins.InstalledRelease{}, false, storageError(err)
		}
		stored, err := q.GetInstalledResource(ctx, sqlc.GetInstalledResourceParams{PluginID: string(in.Release.PluginID), Version: in.Release.Version, ArtifactDigest: in.Release.ArtifactDigest, ResourcePath: resource.Path})
		if err != nil || stored.Sha256Digest != resource.SHA256 || stored.ByteSize != resource.Size || !bytes.Equal(stored.Content, resource.Content) {
			return plugins.InstalledRelease{}, false, plugins.ErrResourceInvalid
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return plugins.InstalledRelease{}, false, plugins.ErrStorage
	}
	mapped, err := mapRelease(row)
	return mapped, created, err
}
func (r *Repository) GetRelease(ctx context.Context, id plugins.PluginID, version string) (plugins.InstalledRelease, error) {
	row, err := r.q.GetInstalledRelease(ctx, sqlc.GetInstalledReleaseParams{PluginID: string(id), Version: version})
	if err != nil {
		return plugins.InstalledRelease{}, storageError(err)
	}
	return mapRelease(row)
}

func (r *Repository) GetResource(ctx context.Context, release plugins.ReleaseIdentity, resourcePath string) (plugins.InstalledResource, error) {
	row, err := r.q.GetInstalledResource(ctx, sqlc.GetInstalledResourceParams{PluginID: string(release.PluginID), Version: release.Version, ArtifactDigest: release.ArtifactDigest, ResourcePath: resourcePath})
	if err != nil {
		return plugins.InstalledResource{}, storageError(err)
	}
	out := plugins.InstalledResource{InstallationID: row.InstallationID.String(), Path: row.ResourcePath, SHA256: row.Sha256Digest, Size: row.ByteSize, Content: append([]byte(nil), row.Content...)}
	if int64(len(out.Content)) != out.Size || digest(out.Content) != out.SHA256 {
		return plugins.InstalledResource{}, plugins.ErrResourceInvalid
	}
	return out, nil
}
func (r *Repository) SetReleaseEnabled(ctx context.Context, id string, enabled bool) (plugins.InstalledRelease, error) {
	key, err := uuid(id)
	if err != nil {
		return plugins.InstalledRelease{}, plugins.ErrInvalidRelease
	}
	row, err := r.q.SetInstalledReleaseEnabled(ctx, sqlc.SetInstalledReleaseEnabledParams{InstallationID: key, Enabled: enabled})
	if err != nil {
		return plugins.InstalledRelease{}, storageError(err)
	}
	return mapRelease(row)
}
func (r *Repository) CreateApproval(ctx context.Context, a plugins.Approval) error {
	if err := a.Validate(); err != nil {
		return err
	}
	id, err := uuid(a.ID)
	if err != nil {
		return plugins.ErrInvalidApproval
	}
	authority := pgtype.Text{}
	if a.AuthorityKeyID != "" {
		authority = pgtype.Text{String: a.AuthorityKeyID, Valid: true}
	}
	err = r.q.CreateReleaseApproval(ctx, sqlc.CreateReleaseApprovalParams{ApprovalID: id, PluginID: string(a.Release.PluginID), Version: a.Release.Version, ArtifactDigest: a.Release.ArtifactDigest, Kind: string(a.Kind), AuthorityKeyID: authority, ApprovedAt: pgtype.Timestamptz{Time: a.ApprovedAt, Valid: true}})
	return storageError(err)
}
func (r *Repository) RevokeApproval(ctx context.Context, id string, at time.Time) error {
	key, err := uuid(id)
	if err != nil {
		return plugins.ErrInvalidApproval
	}
	n, err := r.q.RevokeReleaseApproval(ctx, sqlc.RevokeReleaseApprovalParams{ApprovalID: key, RevokedAt: pgtype.Timestamptz{Time: at, Valid: true}})
	if err != nil {
		return storageError(err)
	}
	if n == 0 {
		return plugins.ErrNotFound
	}
	return nil
}
func (r *Repository) FindActiveApproval(ctx context.Context, id plugins.ReleaseIdentity, kind plugins.ApprovalKind, authority string) (plugins.Approval, error) {
	row, err := r.q.FindActiveReleaseApproval(ctx, sqlc.FindActiveReleaseApprovalParams{PluginID: string(id.PluginID), Version: id.Version, ArtifactDigest: id.ArtifactDigest, Kind: string(kind), AuthorityKeyID: pgtype.Text{String: authority, Valid: true}})
	if err != nil {
		return plugins.Approval{}, storageError(err)
	}
	approval := plugins.Approval{ID: row.ApprovalID.String(), Release: id, Kind: plugins.ApprovalKind(row.Kind), AuthorityKeyID: row.AuthorityKeyID.String, ApprovedAt: row.ApprovedAt.Time.UTC()}
	if row.RevokedAt.Valid {
		v := row.RevokedAt.Time.UTC()
		approval.RevokedAt = &v
	}
	if approval.Validate() != nil {
		return plugins.Approval{}, plugins.ErrStorage
	}
	return approval, nil
}

func mapRelease(row sqlc.PluginsInstalledRelease) (plugins.InstalledRelease, error) {
	var manifest plugins.Manifest
	if err := json.Unmarshal(row.Manifest, &manifest); err != nil || manifest.Validate() != nil {
		return plugins.InstalledRelease{}, plugins.ErrStorage
	}
	trust := plugins.TrustLevel(row.RegisteredTrust)
	if trust != plugins.TrustBTGOwned && trust != plugins.TrustBTGApproved && trust != plugins.TrustUnknown {
		return plugins.InstalledRelease{}, plugins.ErrStorage
	}
	state := plugins.InstallationState(row.State)
	if state != plugins.StateInstalled && state != plugins.StateDisabled {
		return plugins.InstalledRelease{}, plugins.ErrStorage
	}
	out := plugins.InstalledRelease{InstallationID: row.InstallationID.String(), Release: plugins.ReleaseIdentity{PluginID: plugins.PluginID(row.PluginID), Version: row.Version, ArtifactDigest: row.ArtifactDigest}, Manifest: manifest, RegisteredTrust: trust, CurrentTrust: trust, State: state, Enabled: row.Enabled, InstalledAt: row.InstalledAt.Time.UTC()}
	if row.SignatureKeyID.Valid {
		out.Signature = &plugins.SignatureEvidence{KeyID: row.SignatureKeyID.String, Value: row.SignatureValue.String, SigningPayload: append([]byte(nil), row.SigningPayload...)}
	}
	if out.Release.Validate() != nil || out.Enabled != (out.State == plugins.StateInstalled) {
		return plugins.InstalledRelease{}, plugins.ErrStorage
	}
	return out, nil
}
func uuid(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	err := id.Scan(value)
	if err != nil || !id.Valid {
		return id, plugins.ErrInvalidRelease
	}
	return id, nil
}
func storageError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return plugins.ErrNotFound
	}
	return plugins.ErrStorage
}

func sameKeyIdentity(a, b plugins.VerificationKey) bool {
	if a.ID != b.ID || a.Purpose != b.Purpose || !bytes.Equal(a.PublicKey, b.PublicKey) || len(a.AllowedPluginIDs) != len(b.AllowedPluginIDs) {
		return false
	}
	for i := range a.AllowedPluginIDs {
		if a.AllowedPluginIDs[i] != b.AllowedPluginIDs[i] {
			return false
		}
	}
	return true
}

func digest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
