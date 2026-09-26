// Package signing owns the Ed25519 Data Integrity boundary for Open Badges.
// It never resolves JSON-LD contexts over the network.
package signing

import (
	"context"
	"crypto/ed25519"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/multiformats/go-multibase"
	"github.com/piprate/json-gold/ld"
	"github.com/trustbloc/did-go/doc/did"
	"github.com/trustbloc/vc-go/dataintegrity/models"
	"github.com/trustbloc/vc-go/dataintegrity/suite/eddsa2022"
)

//go:embed contexts/*.jsonld
var contexts embed.FS

var (
	ErrInvalidKey         = errors.New("invalid Open Badges signing key")
	ErrInvalidProof       = errors.New("invalid Open Badges Data Integrity proof")
	ErrUntrustedMethod    = errors.New("verification method is not authorized by issuer")
	ErrUnsupportedContext = errors.New("unsupported Open Badges JSON-LD context")
)

const (
	VCContext   = "https://www.w3.org/ns/credentials/v2"
	OBContext   = "https://purl.imsglobal.org/spec/ob/v3p0/context-3.0.3.json"
	Cryptosuite = "eddsa-rdfc-2022"
)

// KeyProvider signs opaque digest input. Private bytes never leave the provider.
type KeyProvider interface {
	ID() string
	Controller() string
	PublicKey() ed25519.PublicKey
	Sign(context.Context, []byte) ([]byte, error)
}

type localKey struct {
	id, controller string
	private        ed25519.PrivateKey
	public         ed25519.PublicKey
}

// NewLocalKey accepts a base64url-encoded 32-byte Ed25519 seed. It never
// generates deployment key material implicitly; operators must retain the seed.
func NewLocalKey(id, controller, seedBase64URL string) (KeyProvider, error) {
	u, err := url.Parse(id)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Fragment == "" || strings.Split(id, "#")[0] != controller {
		return nil, ErrInvalidKey
	}
	seed, err := base64.RawURLEncoding.DecodeString(seedBase64URL)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, ErrInvalidKey
	}
	private := ed25519.NewKeyFromSeed(seed)
	public := append(ed25519.PublicKey(nil), private.Public().(ed25519.PublicKey)...)
	return &localKey{id: id, controller: controller, private: private, public: public}, nil
}
func (k *localKey) ID() string                   { return k.id }
func (k *localKey) Controller() string           { return k.controller }
func (k *localKey) PublicKey() ed25519.PublicKey { return append(ed25519.PublicKey(nil), k.public...) }
func (k *localKey) Sign(_ context.Context, data []byte) ([]byte, error) {
	return ed25519.Sign(k.private, data), nil
}
func (*localKey) String() string { return "Open Badges Ed25519 key [redacted]" }

// Multikey is the public Ed25519 verification method. The multicodec prefix
// 0xed01 identifies Ed25519 public key bytes; z is multibase base58-btc.
type Multikey struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

func PublicMultikey(key KeyProvider) (Multikey, error) {
	if key == nil || len(key.PublicKey()) != ed25519.PublicKeySize {
		return Multikey{}, ErrInvalidKey
	}
	value, err := multibase.Encode(multibase.Base58BTC, append([]byte{0xed, 0x01}, key.PublicKey()...))
	if err != nil {
		return Multikey{}, ErrInvalidKey
	}
	return Multikey{ID: key.ID(), Type: "Multikey", Controller: key.Controller(), PublicKeyMultibase: value}, nil
}

// ControllerDocument authorizes only assertionMethod, never authentication.
type ControllerDocument struct {
	Name               string     `json:"name,omitempty"`
	Context            []string   `json:"@context"`
	ID                 string     `json:"id"`
	VerificationMethod []Multikey `json:"verificationMethod"`
	AssertionMethod    []string   `json:"assertionMethod"`
}

func NewControllerDocument(id string, keys []Multikey) (ControllerDocument, error) {
	if len(keys) == 0 {
		return ControllerDocument{}, ErrInvalidKey
	}
	doc := ControllerDocument{Context: []string{"https://www.w3.org/ns/cid/v1", VCContext}, ID: id}
	seen := map[string]bool{}
	for _, key := range keys {
		if key.Controller != id || key.Type != "Multikey" || seen[key.ID] {
			return ControllerDocument{}, ErrInvalidKey
		}
		seen[key.ID] = true
		doc.VerificationMethod = append(doc.VerificationMethod, key)
		doc.AssertionMethod = append(doc.AssertionMethod, key.ID)
	}
	return doc, nil
}
func (d ControllerDocument) Authorized(id string) (Multikey, bool) {
	for _, allowed := range d.AssertionMethod {
		if allowed == id {
			for _, key := range d.VerificationMethod {
				if key.ID == id && key.Controller == d.ID {
					return key, true
				}
			}
		}
	}
	return Multikey{}, false
}

// Proof is a strictly scoped eddsa-rdfc-2022 assertion proof.
type Proof struct {
	Type               string `json:"type"`
	Cryptosuite        string `json:"cryptosuite"`
	Created            string `json:"created"`
	VerificationMethod string `json:"verificationMethod"`
	ProofPurpose       string `json:"proofPurpose"`
	ProofValue         string `json:"proofValue"`
}

type allowedLoader struct{}

func (allowedLoader) LoadDocument(uri string) (*ld.RemoteDocument, error) {
	var path string
	switch uri {
	case VCContext:
		path = "contexts/vc-v2.jsonld"
	case OBContext:
		path = "contexts/ob3-3.0.3.jsonld"
	default:
		return nil, ErrUnsupportedContext
	}
	bytes, err := contexts.ReadFile(path)
	if err != nil {
		return nil, ErrUnsupportedContext
	}
	var value any
	if json.Unmarshal(bytes, &value) != nil {
		return nil, ErrUnsupportedContext
	}
	return &ld.RemoteDocument{DocumentURL: uri, Document: value}, nil
}

// Sign uses the maintained TrustBloc eddsa-rdfc-2022 cryptosuite, which
// canonicalizes JSON-LD rather than signing serialization bytes.
func Sign(ctx context.Context, unsecured []byte, created time.Time, key KeyProvider) ([]byte, error) {
	if key == nil || created.IsZero() || len(key.PublicKey()) != ed25519.PublicKeySize {
		return nil, ErrInvalidKey
	}
	if err := checkContexts(unsecured); err != nil {
		return nil, err
	}
	vm := did.NewVerificationMethodFromBytes(key.ID(), "Multikey", key.Controller(), key.PublicKey())
	init := eddsa2022.NewSignerInitializer(&eddsa2022.SignerInitializerOptions{LDDocumentLoader: allowedLoader{}, SignerGetter: eddsa2022.WithStaticSigner(providerSigner{ctx, key})})
	suite, err := init.Signer()
	if err != nil {
		return nil, ErrInvalidProof
	}
	proof, err := suite.CreateProof(unsecured, &models.ProofOptions{VerificationMethod: vm, VerificationMethodID: key.ID(), SuiteType: Cryptosuite, ProofType: models.DataIntegrityProof, Purpose: "assertionMethod", Created: created.UTC()})
	if err != nil {
		return nil, ErrInvalidProof
	}
	encoded, err := json.Marshal(Proof{Type: proof.Type, Cryptosuite: proof.CryptoSuite, Created: proof.Created, VerificationMethod: proof.VerificationMethod, ProofPurpose: proof.ProofPurpose, ProofValue: proof.ProofValue})
	if err != nil {
		return nil, ErrInvalidProof
	}
	var document map[string]json.RawMessage
	if json.Unmarshal(unsecured, &document) != nil || document["proof"] != nil {
		return nil, ErrInvalidProof
	}
	document["proof"] = encoded
	secured, err := json.Marshal(document)
	if err != nil {
		return nil, ErrInvalidProof
	}
	return secured, nil
}

type providerSigner struct {
	ctx context.Context
	key KeyProvider
}

func (p providerSigner) Sign(data []byte) ([]byte, error) { return p.key.Sign(p.ctx, data) }

// Verify accepts only an assertion method in the expected issuer's trusted
// controller document. Cryptographic validity alone is insufficient.
func Verify(secured []byte, controller ControllerDocument) error {
	var document map[string]json.RawMessage
	if json.Unmarshal(secured, &document) != nil {
		return ErrInvalidProof
	}
	var proof Proof
	if json.Unmarshal(document["proof"], &proof) != nil || proof.Type != "DataIntegrityProof" || proof.Cryptosuite != Cryptosuite || proof.ProofPurpose != "assertionMethod" || proof.Created == "" || proof.ProofValue == "" {
		return ErrInvalidProof
	}
	if _, err := time.Parse(time.RFC3339, proof.Created); err != nil {
		return ErrInvalidProof
	}
	method, ok := controller.Authorized(proof.VerificationMethod)
	if !ok {
		return ErrUntrustedMethod
	}
	if !strings.HasPrefix(method.PublicKeyMultibase, "z") {
		return ErrInvalidProof
	}
	_, decoded, err := multibase.Decode(method.PublicKeyMultibase)
	if err != nil || len(decoded) != 34 || decoded[0] != 0xed || decoded[1] != 0x01 {
		return ErrInvalidProof
	}
	var issuer struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(document["issuer"], &issuer) != nil || issuer.ID != controller.ID {
		return ErrUntrustedMethod
	}
	delete(document, "proof")
	unsecured, err := json.Marshal(document)
	if err != nil {
		return ErrInvalidProof
	}
	if err = checkContexts(unsecured); err != nil {
		return err
	}
	vm := did.NewVerificationMethodFromBytes(method.ID, "Multikey", controller.ID, decoded[2:])
	init := eddsa2022.NewVerifierInitializer(&eddsa2022.VerifierInitializerOptions{LDDocumentLoader: allowedLoader{}})
	suite, err := init.Verifier()
	if err != nil {
		return ErrInvalidProof
	}
	parsed := &models.Proof{Type: proof.Type, CryptoSuite: proof.Cryptosuite, Created: proof.Created, VerificationMethod: proof.VerificationMethod, ProofPurpose: proof.ProofPurpose, ProofValue: proof.ProofValue}
	created, _ := time.Parse(time.RFC3339, proof.Created)
	if err = suite.VerifyProof(unsecured, parsed, &models.ProofOptions{VerificationMethod: vm, VerificationMethodID: method.ID, SuiteType: Cryptosuite, ProofType: models.DataIntegrityProof, Purpose: "assertionMethod", Created: created}); err != nil {
		return ErrInvalidProof
	}
	return nil
}
func checkContexts(doc []byte) error {
	var obj struct {
		Context []string `json:"@context"`
	}
	if json.Unmarshal(doc, &obj) != nil || len(obj.Context) == 0 || obj.Context[0] != VCContext {
		return ErrUnsupportedContext
	}
	if len(obj.Context) > 2 || len(obj.Context) == 2 && obj.Context[1] != OBContext {
		return ErrUnsupportedContext
	}
	var value any
	if json.Unmarshal(doc, &value) != nil {
		return ErrUnsupportedContext
	}
	options := ld.NewJsonLdOptions("")
	options.ProcessingMode = ld.JsonLd_1_1
	options.DocumentLoader = allowedLoader{}
	options.SafeMode = true
	if _, err := ld.NewJsonLdProcessor().Expand(value, options); err != nil {
		return ErrUnsupportedContext
	}
	return nil
}

// Check that key identities and document claims refer to the same issuer.
func ValidateIssuer(issuer string, key KeyProvider) error {
	if key == nil || issuer != key.Controller() {
		return fmt.Errorf("issuer and signing key mismatch: %w", ErrInvalidKey)
	}
	return nil
}
