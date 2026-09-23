package openbadges

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
)

const testListID = "99999999-9999-4999-8999-999999999999"

func fact(index int, status credentials.CertificateStatus) StatusFact {
	return StatusFact{Entry: StatusListEntry{CertificateID: "11111111-1111-4111-8111-111111111111", StatusListID: testListID, StatusListIndex: index, StatusPurpose: RevocationPurpose}, CertificateStatus: status, CertificatePresent: true}
}
func decodeList(t *testing.T, encoded string) []byte {
	t.Helper()
	if !strings.HasPrefix(encoded, "u") {
		t.Fatalf("wrong multibase prefix: %q", encoded[:1])
	}
	compressed, err := base64.RawURLEncoding.DecodeString(encoded[1:])
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestStatusListBitBoundariesAndDeterminism(t *testing.T) {
	list := StatusList{ID: testListID, Capacity: DefaultStatusListCapacity, StatusPurpose: RevocationPurpose}
	indices := []int{0, 1, 7, 8, 9, DefaultStatusListCapacity - 1}
	facts := make([]StatusFact, 0, len(indices))
	for i, index := range indices {
		item := fact(index, credentials.CertificateRevoked)
		item.Entry.CertificateID = string(rune('1'+i)) + "1111111-1111-4111-8111-111111111111"
		facts = append(facts, item)
	}
	first, err := BuildEncodedList(list, facts)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildEncodedList(list, facts)
	if err != nil || second != first {
		t.Fatal("nondeterministic encoding", err)
	}
	raw := decodeList(t, first)
	if len(raw) != DefaultStatusListCapacity/8 || raw[0] != 0xc1 || raw[1] != 0xc0 || raw[len(raw)-1] != 0x01 {
		t.Fatalf("incorrect W3C MSB bit ordering: first=%x second=%x last=%x length=%d", raw[0], raw[1], raw[len(raw)-1], len(raw))
	}
	for _, b := range raw[2 : len(raw)-1] {
		if b != 0 {
			t.Fatal("unexpected revoked bit")
		}
	}
}
func TestStatusListLifecycleAndFailures(t *testing.T) {
	list := StatusList{ID: testListID, Capacity: DefaultStatusListCapacity, StatusPurpose: RevocationPurpose}
	active := fact(42, credentials.CertificateActive)
	before, err := BuildEncodedList(list, []StatusFact{active})
	if err != nil {
		t.Fatal(err)
	}
	afterFact := active
	afterFact.CertificateStatus = credentials.CertificateRevoked
	after, err := BuildEncodedList(list, []StatusFact{afterFact})
	if err != nil {
		t.Fatal(err)
	}
	original, revoked := decodeList(t, before), decodeList(t, after)
	if original[5] != 0 || revoked[5] != 0x20 {
		t.Fatalf("incorrect status bit: %x %x", original[5], revoked[5])
	}
	active.CertificatePresent = false
	if _, err = BuildEncodedList(list, []StatusFact{active}); !errors.Is(err, ErrMissingStatusCertificate) {
		t.Fatal(err)
	}
	invalid := fact(0, "BROKEN")
	if _, err = BuildEncodedList(list, []StatusFact{invalid}); !errors.Is(err, ErrInvalidStatusList) {
		t.Fatal(err)
	}
	if _, err = BuildEncodedList(list, []StatusFact{fact(0, credentials.CertificateActive), fact(0, credentials.CertificateRevoked)}); !errors.Is(err, ErrInvalidStatusList) {
		t.Fatal(err)
	}
}
func TestUnsignedStatusListCredentialIsInternalProoflessAndPrivate(t *testing.T) {
	mapper, err := NewMapper(Config{PublicBaseURL: "https://academy.example", SubjectSalt: []byte(strings.Repeat("a", 32))})
	if err != nil {
		t.Fatal(err)
	}
	list := StatusList{ID: testListID, Capacity: DefaultStatusListCapacity, StatusPurpose: RevocationPurpose}
	snapshot, err := mapper.BuildUnsignedStatusListCredential(list, []StatusFact{fact(7, credentials.CertificateRevoked)}, credentials.Issuer{ID: "https://academy.example/issuer", Name: "Academy"}, time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Context[0] != VCContext || snapshot.Type[1] != "BitstringStatusListCredential" || snapshot.CredentialSubject.Type != "BitstringStatusList" || snapshot.CredentialSubject.StatusPurpose != RevocationPurpose || snapshot.CredentialSubject.ID != snapshot.ID+"#list" || snapshot.ValidFrom != "2025-01-02T03:04:05Z" {
		t.Fatalf("invalid status list VC: %s", raw)
	}
	for _, forbidden := range []string{"proof", "proofValue", "verificationMethod", "cryptosuite", "11111111-1111-4111-8111-111111111111", "learner", "subjectSecret"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("private or proof field %q in %s", forbidden, raw)
		}
	}
	if decodeList(t, snapshot.CredentialSubject.EncodedList)[0] != 0x01 {
		t.Fatal("wrong encoded bit")
	}
}
