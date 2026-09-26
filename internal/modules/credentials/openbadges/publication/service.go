package publication

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	statuspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/signing"
)

var (
	ErrUnavailable                   = errors.New("signed Open Badges resource unavailable")
	ErrRevocationPublicationFailed   = errors.New("certificate revoked but portable status publication failed")
	ErrRevocationCertificateNotFound = errors.New("certificate unavailable for revocation")
)

type CertificateReader interface {
	GetByID(context.Context, credentials.CertificateID) (credentials.Certificate, error)
}
type CertificateRevoker interface {
	CertificateReader
	Revoke(context.Context, credentials.CertificateID, time.Time) (credentials.Certificate, error)
}
type Service struct {
	certificates CertificateReader
	status       *statuspostgres.Repository
	mapper       *openbadges.Mapper
	key          signing.KeyProvider
	issuer       credentials.Issuer
	now          func() time.Time
	logger       *slog.Logger
}

// WithLogger adds operational diagnostics using only public key/list/certificate
// identifiers. It never logs signing or pseudonym secret material.
func (s *Service) WithLogger(logger *slog.Logger) *Service {
	if s == nil {
		return nil
	}
	copy := *s
	copy.logger = logger
	return &copy
}

func NewService(certificates CertificateReader, status *statuspostgres.Repository, mapper *openbadges.Mapper, key signing.KeyProvider, issuer credentials.Issuer, now func() time.Time) (*Service, error) {
	if certificates == nil || status == nil || mapper == nil || key == nil || now == nil || signing.ValidateIssuer(issuer.ID, key) != nil {
		return nil, openbadges.ErrInvalidConfiguration
	}
	return &Service{certificates: certificates, status: status, mapper: mapper, key: key, issuer: issuer, now: now}, nil
}
func (s *Service) RegisterKey(ctx context.Context) error {
	method, err := signing.PublicMultikey(s.key)
	if err != nil {
		return err
	}
	return s.status.RegisterMethod(ctx, method)
}
func (s *Service) Controller(ctx context.Context) (signing.ControllerDocument, error) {
	methods, err := s.status.Methods(ctx, s.issuer.ID)
	if err != nil {
		return signing.ControllerDocument{}, err
	}
	doc, err := signing.NewControllerDocument(s.issuer.ID, methods)
	doc.Name = s.issuer.Name
	return doc, err
}
func (s *Service) Status(ctx context.Context, listID string) ([]byte, error) {
	// A signed snapshot is served only if its revision still matches durable
	// lifecycle state. On signing failure after revocation, fail closed.
	current, err := s.status.CurrentSnapshot(ctx, listID)
	if err == nil {
		if s.verify(ctx, current.Document) == nil {
			return current.Document, nil
		}
		s.logFailure("verify_status_snapshot", listID)
		return nil, ErrUnavailable
	}
	if !errors.Is(err, openbadges.ErrStatusEntryNotFound) {
		return nil, ErrUnavailable
	}
	generated, err := s.status.PublishCurrentSnapshot(ctx, listID, func(list openbadges.StatusList, facts []openbadges.StatusFact) (statuspostgres.PublishedSnapshot, error) {
		at := s.now().UTC()
		unsigned, err := s.mapper.BuildUnsignedStatusListCredential(list, facts, s.issuer, at)
		if err != nil {
			return statuspostgres.PublishedSnapshot{}, err
		}
		raw, err := json.Marshal(unsigned)
		if err != nil {
			return statuspostgres.PublishedSnapshot{}, err
		}
		signed, err := signing.Sign(ctx, raw, at, s.key)
		if err != nil {
			return statuspostgres.PublishedSnapshot{}, err
		}
		if err = s.verify(ctx, signed); err != nil {
			return statuspostgres.PublishedSnapshot{}, err
		}
		return statuspostgres.PublishedSnapshot{Document: signed, EncodedList: unsigned.CredentialSubject.EncodedList, KeyID: s.key.ID(), GeneratedAt: at, LifecycleRevision: list.LifecycleRevision}, nil
	})
	if errors.Is(err, openbadges.ErrStatusEntryNotFound) {
		return nil, err
	}
	if err != nil {
		s.logFailure("publish_status_snapshot", listID)
		return nil, ErrUnavailable
	}
	return generated.Document, nil
}
func (s *Service) Credential(ctx context.Context, id credentials.CertificateID) ([]byte, error) {
	certificate, err := s.certificates.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if signing.ValidateIssuer(certificate.Issuer.ID, s.key) != nil {
		return nil, ErrUnavailable
	}
	entry, err := s.status.EnsureEntry(ctx, string(id))
	if err != nil {
		return nil, ErrUnavailable
	}
	if _, err = s.Status(ctx, entry.StatusListID); err != nil {
		return nil, ErrUnavailable
	}
	existing, err := s.status.GetSignedCredential(ctx, string(id))
	if err == nil {
		if s.verify(ctx, existing) == nil {
			return existing, nil
		}
		return nil, ErrUnavailable
	}
	if !errors.Is(err, openbadges.ErrStatusEntryNotFound) {
		return nil, ErrUnavailable
	}
	unsigned, err := s.mapper.MapForSigning(certificate)
	if err != nil {
		return nil, ErrUnavailable
	}
	reference, err := s.mapper.StatusReferenceFor(entry)
	if err != nil {
		return nil, ErrUnavailable
	}
	unsigned.CredentialStatus = &reference
	raw, err := json.Marshal(unsigned)
	if err != nil {
		return nil, ErrUnavailable
	}
	created := s.now().UTC() // actual proof creation, not historical Certificate issuance
	signed, err := signing.Sign(ctx, raw, created, s.key)
	if err != nil {
		return nil, ErrUnavailable
	}
	if err = s.verify(ctx, signed); err != nil {
		return nil, ErrUnavailable
	}
	persisted, err := s.status.SaveSignedCredential(ctx, string(id), s.key.ID(), created, signed)
	if err != nil {
		return nil, ErrUnavailable
	}
	return persisted, nil
}

// Revoke is the only signed-Open-Badges revocation orchestration entry point.
// It first ensures a current ACTIVE list exists, persists the one-way
// Certificate lifecycle transition, and immediately publishes a complete signed
// snapshot for its stable entry. If the final publication fails, the durable
// Certificate remains revoked but this method returns an explicit failure and
// Status fails closed until a retry completes the snapshot.
//
// PostgreSQL cannot make the externally signed bytes and the Certificate write
// one transaction without holding a signing operation inside a cross-module
// credential transaction. This recovery-safe sequence therefore makes partial
// publication observable and retryable rather than reporting false success.
func (s *Service) Revoke(ctx context.Context, certificates CertificateRevoker, id credentials.CertificateID, revokedAt time.Time) (credentials.Certificate, error) {
	if s == nil || certificates == nil || revokedAt.IsZero() {
		return credentials.Certificate{}, ErrRevocationCertificateNotFound
	}
	certificate, err := certificates.GetByID(ctx, id)
	if errors.Is(err, credentials.ErrCertificateNotFound) {
		return credentials.Certificate{}, ErrRevocationCertificateNotFound
	}
	if err != nil {
		return credentials.Certificate{}, ErrUnavailable
	}
	entry, err := s.status.EnsureEntry(ctx, string(id))
	if err != nil {
		return credentials.Certificate{}, ErrRevocationPublicationFailed
	}
	// Establish a signed baseline before changing lifecycle state. Failure here
	// leaves the Certificate ACTIVE and gives the operator a retryable result.
	if _, err = s.Status(ctx, entry.StatusListID); err != nil {
		s.logFailure("publish_revocation_baseline", entry.StatusListID)
		return certificate, ErrRevocationPublicationFailed
	}
	revoked, err := certificates.Revoke(ctx, id, revokedAt)
	if errors.Is(err, credentials.ErrCertificateNotFound) {
		return credentials.Certificate{}, ErrRevocationCertificateNotFound
	}
	if err != nil {
		return credentials.Certificate{}, ErrUnavailable
	}
	if _, err = s.Status(ctx, entry.StatusListID); err != nil {
		s.logFailure("publish_revocation", entry.StatusListID)
		return revoked, ErrRevocationPublicationFailed
	}
	return revoked, nil
}
func (s *Service) verify(ctx context.Context, document []byte) error {
	controller, err := s.Controller(ctx)
	if err != nil {
		return ErrUnavailable
	}
	return signing.Verify(document, controller)
}

func (s *Service) logFailure(operation, listID string) {
	if s != nil && s.logger != nil {
		s.logger.Error("Open Badges operation unavailable", "operation", operation, "status_list_id", listID, "key_id", s.key.ID())
	}
}
