package publication

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	statuspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/signing"
)

var ErrUnavailable = errors.New("signed Open Badges resource unavailable")

type CertificateReader interface {
	GetByID(context.Context, credentials.CertificateID) (credentials.Certificate, error)
}
type Service struct {
	certificates CertificateReader
	status       *statuspostgres.Repository
	mapper       *openbadges.Mapper
	key          signing.KeyProvider
	issuer       credentials.Issuer
	now          func() time.Time
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
func (s *Service) verify(ctx context.Context, document []byte) error {
	controller, err := s.Controller(ctx)
	if err != nil {
		return ErrUnavailable
	}
	return signing.Verify(document, controller)
}
