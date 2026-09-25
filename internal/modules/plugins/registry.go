package plugins

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/google/uuid"
)

type TrustLevel string

const (
	TrustBTGOwned    TrustLevel = "BTG_OWNED"
	TrustBTGApproved TrustLevel = "BTG_APPROVED"
	TrustUnknown     TrustLevel = "UNKNOWN"
)

type KeyPurpose string

const (
	KeyPurposeOwned    KeyPurpose = "BTG_OWNED_SIGNING"
	KeyPurposeApproval KeyPurpose = "BTG_APPROVAL_SIGNING"
)

type VerificationKey struct {
	ID               string
	PublicKey        ed25519.PublicKey
	Purpose          KeyPurpose
	AllowedPluginIDs []PluginID
	Enabled          bool
}

func (k VerificationKey) Validate() error {
	if !keyIDPattern.MatchString(k.ID) || len(k.PublicKey) != ed25519.PublicKeySize || (k.Purpose != KeyPurposeOwned && k.Purpose != KeyPurposeApproval) {
		return ErrInvalidKey
	}
	if k.Purpose == KeyPurposeOwned && len(k.AllowedPluginIDs) == 0 {
		return ErrInvalidKey
	}
	seen := map[PluginID]bool{}
	for _, id := range k.AllowedPluginIDs {
		if !pluginIDPattern.MatchString(string(id)) || seen[id] {
			return ErrInvalidKey
		}
		seen[id] = true
	}
	return nil
}

type ApprovalKind string

const (
	ApprovalBTGRelease   ApprovalKind = "BTG_RELEASE_APPROVAL"
	ApprovalLocalUnknown ApprovalKind = "LOCAL_UNKNOWN_APPROVAL"
)

type ReleaseIdentity struct {
	PluginID                PluginID
	Version, ArtifactDigest string
}

func (i ReleaseIdentity) Validate() error {
	if !pluginIDPattern.MatchString(string(i.PluginID)) || !validDigest(i.ArtifactDigest) {
		return ErrInvalidRelease
	}
	return validateVersion(i.Version)
}

type Approval struct {
	ID             string
	Release        ReleaseIdentity
	Kind           ApprovalKind
	AuthorityKeyID string
	ApprovedAt     time.Time
	RevokedAt      *time.Time
}

func (a Approval) Active() bool { return a.RevokedAt == nil }
func (a Approval) Validate() error {
	if a.ID == "" || a.Release.Validate() != nil || a.ApprovedAt.IsZero() {
		return ErrInvalidApproval
	}
	switch a.Kind {
	case ApprovalBTGRelease:
		if !keyIDPattern.MatchString(a.AuthorityKeyID) {
			return ErrInvalidApproval
		}
	case ApprovalLocalUnknown:
		if a.AuthorityKeyID != "" {
			return ErrInvalidApproval
		}
	default:
		return ErrInvalidApproval
	}
	return nil
}

type InstallationState string

const (
	StateInstalled InstallationState = "INSTALLED"
	StateDisabled  InstallationState = "DISABLED"
)

type SignatureEvidence struct {
	KeyID, Value   string
	SigningPayload []byte
}
type InstalledRelease struct {
	InstallationID  string
	Release         ReleaseIdentity
	Manifest        Manifest
	Signature       *SignatureEvidence
	RegisteredTrust TrustLevel
	CurrentTrust    TrustLevel
	State           InstallationState
	Enabled         bool
	InstalledAt     time.Time
}

// InstalledResource is an immutable member of a validated release inventory.
// Content is copied at repository boundaries so callers cannot mutate a
// registered artifact in memory.
type InstalledResource struct {
	InstallationID string
	Path           string
	SHA256         string
	Size           int64
	Content        []byte
}

type Policy struct {
	AllowUnknown                    bool
	RequireManualApprovalForUnknown bool
}

var (
	ErrInvalidKey      = errors.New("invalid plugin verification key")
	ErrKeyIDConflict   = errors.New("plugin verification key ID conflict")
	ErrInvalidRelease  = errors.New("invalid plugin release")
	ErrReleaseConflict = errors.New("plugin release identity conflict")
	ErrInvalidApproval = errors.New("invalid plugin approval")
	ErrNotFound        = errors.New("plugin release not found")
	ErrEnableDenied    = errors.New("plugin enablement denied")
	ErrLaunchDenied    = errors.New("plugin runtime launch denied")
	ErrResourceInvalid = errors.New("plugin resource integrity failure")
	ErrStorage         = errors.New("plugin registry unavailable")
)

type Repository interface {
	RegisterKey(context.Context, VerificationKey) error
	GetKey(context.Context, string) (VerificationKey, error)
	SetKeyEnabled(context.Context, string, bool) error
	RegisterRelease(context.Context, InstalledRelease, []InstalledResource) (InstalledRelease, bool, error)
	GetRelease(context.Context, PluginID, string) (InstalledRelease, error)
	ListReleases(context.Context) ([]InstalledRelease, error)
	GetResource(context.Context, ReleaseIdentity, string) (InstalledResource, error)
	SetReleaseEnabled(context.Context, string, bool) (InstalledRelease, error)
	CreateApproval(context.Context, Approval) error
	RevokeApproval(context.Context, string, time.Time) error
	FindActiveApproval(context.Context, ReleaseIdentity, ApprovalKind, string) (Approval, error)
}

type TrustEvaluator struct{ repository Repository }

func NewTrustEvaluator(r Repository) TrustEvaluator { return TrustEvaluator{repository: r} }
func (e TrustEvaluator) EvaluatePackage(ctx context.Context, p *ValidatedPluginPackage) (TrustLevel, error) {
	if p == nil {
		return "", ErrInvalidRelease
	}
	sig, ok := p.Signature()
	var evidence *SignatureEvidence
	if ok {
		evidence = &SignatureEvidence{KeyID: sig.KeyID, Value: sig.Value, SigningPayload: p.SigningPayload()}
	}
	return e.evaluate(ctx, ReleaseIdentity{p.manifest.ID, p.manifest.Version, p.artifactDigest}, evidence)
}
func (e TrustEvaluator) EvaluateRelease(ctx context.Context, r InstalledRelease) (TrustLevel, error) {
	return e.evaluate(ctx, r.Release, r.Signature)
}
func (e TrustEvaluator) evaluate(ctx context.Context, id ReleaseIdentity, sig *SignatureEvidence) (TrustLevel, error) {
	if id.Validate() != nil {
		return "", ErrInvalidRelease
	}
	if sig == nil {
		return TrustUnknown, nil
	}
	key, err := e.repository.GetKey(ctx, sig.KeyID)
	if errors.Is(err, ErrNotFound) {
		return TrustUnknown, nil
	}
	if err != nil {
		return "", err
	}
	if !key.Enabled {
		return TrustUnknown, nil
	}
	decoded, err := decodeSignature(sig.Value)
	if err != nil || !ed25519.Verify(key.PublicKey, sig.SigningPayload, decoded) {
		return "", packageError(ErrInvalidSignature, "signature.json")
	}
	switch key.Purpose {
	case KeyPurposeOwned:
		for _, allowed := range key.AllowedPluginIDs {
			if allowed == id.PluginID {
				return TrustBTGOwned, nil
			}
		}
		return TrustUnknown, nil
	case KeyPurposeApproval:
		_, err = e.repository.FindActiveApproval(ctx, id, ApprovalBTGRelease, key.ID)
		if errors.Is(err, ErrNotFound) {
			return TrustUnknown, nil
		}
		if err != nil {
			return "", err
		}
		return TrustBTGApproved, nil
	default:
		return "", ErrInvalidKey
	}
}

type RegistryService struct {
	repository Repository
	trust      TrustEvaluator
	policy     Policy
	now        func() time.Time
}

func NewRegistryService(r Repository, p Policy) *RegistryService {
	return &RegistryService{repository: r, trust: NewTrustEvaluator(r), policy: p, now: func() time.Time { return time.Now().UTC() }}
}
func (s *RegistryService) SetKeyEnabled(ctx context.Context, id string, enabled bool) error {
	if !keyIDPattern.MatchString(id) {
		return ErrInvalidKey
	}
	return s.repository.SetKeyEnabled(ctx, id, enabled)
}
func (s *RegistryService) Register(ctx context.Context, p *ValidatedPluginPackage) (InstalledRelease, bool, error) {
	if s == nil || s.repository == nil || p == nil {
		return InstalledRelease{}, false, ErrInvalidRelease
	}
	trust, err := s.trust.EvaluatePackage(ctx, p)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	sig, ok := p.Signature()
	var evidence *SignatureEvidence
	if ok {
		evidence = &SignatureEvidence{KeyID: sig.KeyID, Value: sig.Value, SigningPayload: p.SigningPayload()}
	}
	release := InstalledRelease{InstallationID: uuid.NewString(), Release: ReleaseIdentity{p.manifest.ID, p.manifest.Version, p.artifactDigest}, Manifest: p.Manifest(), Signature: evidence, RegisteredTrust: trust, CurrentTrust: trust, State: StateDisabled, InstalledAt: s.now()}
	resources := make([]InstalledResource, 0, len(p.manifest.Resources))
	for _, declared := range p.manifest.Resources {
		resources = append(resources, InstalledResource{InstallationID: release.InstallationID, Path: declared.Path, SHA256: declared.SHA256, Size: declared.Size, Content: append([]byte(nil), p.resources[declared.Path]...)})
	}
	stored, created, err := s.repository.RegisterRelease(ctx, release, resources)
	if err != nil {
		return InstalledRelease{}, false, err
	}
	stored.CurrentTrust, err = s.trust.EvaluateRelease(ctx, stored)
	return stored, created, err
}

// RuntimeRelease re-evaluates trust and local policy for every launch or token
// refresh. Enabled is necessary but never sufficient on its own.
func (s *RegistryService) RuntimeRelease(ctx context.Context, id PluginID, version string) (InstalledRelease, error) {
	r, err := s.Get(ctx, id, version)
	if err != nil {
		return InstalledRelease{}, err
	}
	if !r.Enabled || !s.executionAllowed(ctx, r) {
		return InstalledRelease{}, ErrLaunchDenied
	}
	return r, nil
}

func (s *RegistryService) Resource(ctx context.Context, release ReleaseIdentity, resourcePath string) (InstalledResource, error) {
	if release.Validate() != nil {
		return InstalledResource{}, ErrInvalidRelease
	}
	return s.repository.GetResource(ctx, release, resourcePath)
}

func (s *RegistryService) executionAllowed(ctx context.Context, r InstalledRelease) bool {
	if r.CurrentTrust == TrustBTGOwned || r.CurrentTrust == TrustBTGApproved {
		return true
	}
	if r.CurrentTrust != TrustUnknown || !s.policy.AllowUnknown {
		return false
	}
	if !s.policy.RequireManualApprovalForUnknown {
		return true
	}
	_, err := s.repository.FindActiveApproval(ctx, r.Release, ApprovalLocalUnknown, "")
	return err == nil
}
func (s *RegistryService) Get(ctx context.Context, id PluginID, version string) (InstalledRelease, error) {
	r, err := s.repository.GetRelease(ctx, id, version)
	if err != nil {
		return InstalledRelease{}, err
	}
	r.CurrentTrust, err = s.trust.EvaluateRelease(ctx, r)
	return r, err
}

// CourseWidgets returns only releases currently permitted for Course
// composition. It deliberately re-evaluates trust instead of returning a
// registry-time classification.
func (s *RegistryService) CourseWidgets(ctx context.Context) ([]InstalledRelease, error) {
	if s == nil || s.repository == nil {
		return nil, ErrStorage
	}
	releases, err := s.repository.ListReleases(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]InstalledRelease, 0, len(releases))
	for _, release := range releases {
		current, err := s.Get(ctx, release.Release.PluginID, release.Release.Version)
		if err != nil {
			return nil, err
		}
		if !current.Enabled || !s.executionAllowed(ctx, current) {
			continue
		}
		for _, entry := range current.Manifest.Entrypoints {
			if entry.Type == TypeCourseWidget {
				result = append(result, current)
				break
			}
		}
	}
	return result, nil
}
func (s *RegistryService) Enable(ctx context.Context, id PluginID, version string) (InstalledRelease, error) {
	r, err := s.Get(ctx, id, version)
	if err != nil {
		return InstalledRelease{}, err
	}
	allowed := r.CurrentTrust == TrustBTGOwned || r.CurrentTrust == TrustBTGApproved
	if r.CurrentTrust == TrustUnknown && s.policy.AllowUnknown {
		allowed = !s.policy.RequireManualApprovalForUnknown
		if !allowed {
			_, err = s.repository.FindActiveApproval(ctx, r.Release, ApprovalLocalUnknown, "")
			allowed = err == nil
			if err != nil && !errors.Is(err, ErrNotFound) {
				return InstalledRelease{}, err
			}
		}
	}
	if !allowed {
		return InstalledRelease{}, ErrEnableDenied
	}
	r, err = s.repository.SetReleaseEnabled(ctx, r.InstallationID, true)
	r.CurrentTrust = r.RegisteredTrust
	if err == nil {
		r.CurrentTrust, _ = s.trust.EvaluateRelease(ctx, r)
	}
	return r, err
}
func (s *RegistryService) Disable(ctx context.Context, id PluginID, version string) (InstalledRelease, error) {
	r, err := s.repository.GetRelease(ctx, id, version)
	if err != nil {
		return InstalledRelease{}, err
	}
	return s.repository.SetReleaseEnabled(ctx, r.InstallationID, false)
}
func (s *RegistryService) Approve(ctx context.Context, release ReleaseIdentity, kind ApprovalKind, authority string) (Approval, error) {
	a := Approval{ID: uuid.NewString(), Release: release, Kind: kind, AuthorityKeyID: authority, ApprovedAt: s.now()}
	if err := a.Validate(); err != nil {
		return Approval{}, err
	}
	registered, err := s.repository.GetRelease(ctx, release.PluginID, release.Version)
	if err != nil || registered.Release.ArtifactDigest != release.ArtifactDigest {
		if err != nil {
			return Approval{}, err
		}
		return Approval{}, ErrInvalidApproval
	}
	if kind == ApprovalBTGRelease {
		key, err := s.repository.GetKey(ctx, authority)
		if err != nil {
			return Approval{}, err
		}
		if !key.Enabled || key.Purpose != KeyPurposeApproval || registered.Signature == nil || registered.Signature.KeyID != authority {
			return Approval{}, ErrInvalidApproval
		}
		signature, decodeErr := decodeSignature(registered.Signature.Value)
		if decodeErr != nil || !ed25519.Verify(key.PublicKey, registered.Signature.SigningPayload, signature) {
			return Approval{}, ErrInvalidApproval
		}
	}
	if err := s.repository.CreateApproval(ctx, a); err != nil {
		return Approval{}, err
	}
	return a, nil
}
func (s *RegistryService) RevokeApproval(ctx context.Context, id string) error {
	return s.repository.RevokeApproval(ctx, id, s.now())
}

func validateVersion(v string) error {
	_, err := courses.ParseVersion(v)
	if err != nil {
		return ErrInvalidRelease
	}
	return nil
}
func decodeSignature(v string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(v) }
