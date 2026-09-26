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
	guuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

var _ plugins.Repository = (*Repository)(nil)
var _ plugins.DashboardPlacementRepository = (*Repository)(nil)

func New(db *pgxpool.Pool) *Repository { return &Repository{pool: db, q: sqlc.New(db)} }

func (r *Repository) ListDashboardPlacements(ctx context.Context) ([]plugins.DashboardPlacement, error) {
	rows, err := r.q.ListDashboardWidgetPlacements(ctx)
	if err != nil {
		return nil, storageError(err)
	}
	result := make([]plugins.DashboardPlacement, 0, len(rows))
	for _, row := range rows {
		value, err := dashboardPlacement(row)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}
func (r *Repository) GetDashboardPlacement(ctx context.Context, id string) (plugins.DashboardPlacement, error) {
	value, err := uuid(id)
	if err != nil {
		return plugins.DashboardPlacement{}, plugins.ErrDashboardPlacementNotFound
	}
	row, err := r.q.GetDashboardWidgetPlacement(ctx, value)
	if errors.Is(err, pgx.ErrNoRows) {
		return plugins.DashboardPlacement{}, plugins.ErrDashboardPlacementNotFound
	}
	if err != nil {
		return plugins.DashboardPlacement{}, storageError(err)
	}
	return dashboardPlacement(row)
}
func (r *Repository) CreateDashboardPlacement(ctx context.Context, value plugins.DashboardPlacement) (plugins.DashboardPlacement, error) {
	id, err := uuid(value.ID)
	if err != nil {
		return plugins.DashboardPlacement{}, plugins.ErrStorage
	}
	row, err := r.q.CreateDashboardWidgetPlacement(ctx, sqlc.CreateDashboardWidgetPlacementParams{PlacementID: id, PluginID: string(value.PluginID), PluginVersion: value.PluginVersion, ArtifactDigest: value.ArtifactDigest, WidgetID: value.WidgetID, Configuration: value.Configuration, Enabled: value.Enabled, Revision: value.Revision, CreatedAt: pgtype.Timestamptz{Time: value.CreatedAt, Valid: true}, UpdatedAt: pgtype.Timestamptz{Time: value.UpdatedAt, Valid: true}})
	if err != nil {
		return plugins.DashboardPlacement{}, storageError(err)
	}
	return dashboardPlacement(row)
}
func (r *Repository) UpdateDashboardPlacement(ctx context.Context, value plugins.DashboardPlacement, expected int64) (plugins.DashboardPlacement, error) {
	id, err := uuid(value.ID)
	if err != nil {
		return plugins.DashboardPlacement{}, plugins.ErrDashboardPlacementNotFound
	}
	row, err := r.q.UpdateDashboardWidgetPlacement(ctx, sqlc.UpdateDashboardWidgetPlacementParams{PlacementID: id, Configuration: value.Configuration, Enabled: value.Enabled, UpdatedAt: pgtype.Timestamptz{Time: value.UpdatedAt, Valid: true}, Revision: expected})
	if errors.Is(err, pgx.ErrNoRows) {
		return plugins.DashboardPlacement{}, plugins.ErrDashboardPlacementConflict
	}
	if err != nil {
		return plugins.DashboardPlacement{}, storageError(err)
	}
	return dashboardPlacement(row)
}
func (r *Repository) DeleteDashboardPlacement(ctx context.Context, id string, expected int64) error {
	value, err := uuid(id)
	if err != nil {
		return plugins.ErrDashboardPlacementNotFound
	}
	n, err := r.q.DeleteDashboardWidgetPlacement(ctx, sqlc.DeleteDashboardWidgetPlacementParams{PlacementID: value, Revision: expected})
	if err != nil {
		return storageError(err)
	}
	if n == 0 {
		return plugins.ErrDashboardPlacementConflict
	}
	return nil
}
func (r *Repository) ReorderDashboardPlacements(ctx context.Context, ids []string, revisions map[string]int64, updated time.Time) ([]plugins.DashboardPlacement, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, storageError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT placement_id::text, revision FROM plugins.dashboard_widget_placement ORDER BY position FOR UPDATE`)
	if err != nil {
		return nil, storageError(err)
	}
	defer rows.Close()
	existing := map[string]int64{}
	for rows.Next() {
		var id string
		var revision int64
		if err := rows.Scan(&id, &revision); err != nil {
			return nil, storageError(err)
		}
		existing[id] = revision
	}
	if rows.Err() != nil || len(existing) != len(ids) {
		return nil, plugins.ErrDashboardPlacementConflict
	}
	for _, id := range ids {
		if existing[id] != revisions[id] {
			return nil, plugins.ErrDashboardPlacementConflict
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE plugins.dashboard_widget_placement SET position = position + 1000000`); err != nil {
		return nil, storageError(err)
	}
	for position, id := range ids {
		if _, err = tx.Exec(ctx, `UPDATE plugins.dashboard_widget_placement SET position=$1, revision=revision+1, updated_at=$2 WHERE placement_id=$3`, position, updated, id); err != nil {
			return nil, storageError(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, storageError(err)
	}
	return r.ListDashboardPlacements(ctx)
}
func dashboardPlacement(row sqlc.PluginsDashboardWidgetPlacement) (plugins.DashboardPlacement, error) {
	if !row.PlacementID.Valid || !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		return plugins.DashboardPlacement{}, plugins.ErrStorage
	}
	id := guuid.UUID(row.PlacementID.Bytes).String()
	return plugins.DashboardPlacement{ID: id, PluginID: plugins.PluginID(row.PluginID), PluginVersion: row.PluginVersion, ArtifactDigest: row.ArtifactDigest, WidgetID: row.WidgetID, Configuration: append(json.RawMessage(nil), row.Configuration...), Position: int(row.Position), Enabled: row.Enabled, Revision: row.Revision, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time}, nil
}

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
func (r *Repository) ListKeys(ctx context.Context) ([]plugins.VerificationKey, error) {
	rows, err := r.q.ListVerificationKeys(ctx)
	if err != nil {
		return nil, storageError(err)
	}
	keys := make([]plugins.VerificationKey, 0, len(rows))
	for _, row := range rows {
		allowed := make([]plugins.PluginID, len(row.AllowedPluginIds))
		for i, value := range row.AllowedPluginIds {
			allowed[i] = plugins.PluginID(value)
		}
		key := plugins.VerificationKey{ID: row.KeyID, PublicKey: ed25519.PublicKey(append([]byte(nil), row.PublicKey...)), Purpose: plugins.KeyPurpose(row.Purpose), AllowedPluginIDs: allowed, Enabled: row.Enabled}
		if key.Validate() != nil {
			return nil, plugins.ErrStorage
		}
		keys = append(keys, key)
	}
	return keys, nil
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

func (r *Repository) ListReleases(ctx context.Context) ([]plugins.InstalledRelease, error) {
	rows, err := r.q.ListInstalledReleases(ctx)
	if err != nil {
		return nil, storageError(err)
	}
	result := make([]plugins.InstalledRelease, 0, len(rows))
	for _, row := range rows {
		release, err := mapRelease(row)
		if err != nil {
			return nil, err
		}
		result = append(result, release)
	}
	return result, nil
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return plugins.ErrApprovalConflict
	}
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
func (r *Repository) ListApprovals(ctx context.Context, id plugins.ReleaseIdentity) ([]plugins.Approval, error) {
	rows, err := r.q.ListReleaseApprovals(ctx, sqlc.ListReleaseApprovalsParams{PluginID: string(id.PluginID), Version: id.Version, ArtifactDigest: id.ArtifactDigest})
	if err != nil {
		return nil, storageError(err)
	}
	approvals := make([]plugins.Approval, 0, len(rows))
	for _, row := range rows {
		approval := plugins.Approval{ID: row.ApprovalID.String(), Release: id, Kind: plugins.ApprovalKind(row.Kind), AuthorityKeyID: row.AuthorityKeyID.String, ApprovedAt: row.ApprovedAt.Time.UTC()}
		if row.RevokedAt.Valid {
			at := row.RevokedAt.Time.UTC()
			approval.RevokedAt = &at
		}
		if approval.Validate() != nil {
			return nil, plugins.ErrStorage
		}
		approvals = append(approvals, approval)
	}
	return approvals, nil
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
