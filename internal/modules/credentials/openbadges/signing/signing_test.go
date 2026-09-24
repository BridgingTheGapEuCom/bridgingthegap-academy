package signing_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/signing"
)

const document = `{"@context":["https://www.w3.org/ns/credentials/v2"],"id":"https://academy.example/status/1","type":["VerifiableCredential","BitstringStatusListCredential"],"issuer":{"id":"https://academy.example/issuer","type":["Profile"],"name":"Academy"},"validFrom":"2026-09-23T12:00:00Z","credentialSubject":{"id":"https://academy.example/status/1#list","type":"BitstringStatusList","statusPurpose":"revocation","encodedList":"uH4sIAAAAAAAA"}}`

func key(t *testing.T) signing.KeyProvider {
	t.Helper()
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	key, err := signing.NewLocalKey("https://academy.example/issuer#key-1", "https://academy.example/issuer", base64.RawURLEncoding.EncodeToString(seed))
	if err != nil {
		t.Fatal(err)
	}
	return key
}
func TestSignAndVerify(t *testing.T) {
	k := key(t)
	method, err := signing.PublicMultikey(k)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := signing.NewControllerDocument(k.Controller(), []signing.Multikey{method})
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 9, 23, 12, 30, 0, 0, time.UTC)
	first, err := signing.Sign(context.Background(), []byte(document), created, k)
	if err != nil {
		t.Fatal(err)
	}
	second, err := signing.Sign(context.Background(), []byte(document), created, k)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("nondeterministic: %v", err)
	}
	if err = signing.Verify(first, controller); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), `"cryptosuite":"eddsa-rdfc-2022"`) || strings.Contains(string(first), "secretKey") {
		t.Fatal(string(first))
	}
	for _, field := range []string{"id", "validFrom", "credentialSubject", "issuer"} {
		var obj map[string]json.RawMessage
		if json.Unmarshal(first, &obj) != nil {
			t.Fatal("bad JSON")
		}
		if field == "issuer" {
			obj[field] = json.RawMessage(`{"id":"https://evil.example","type":["Profile"],"name":"Evil"}`)
		} else {
			obj[field] = json.RawMessage(`"https://evil.example"`)
		}
		tampered, _ := json.Marshal(obj)
		if signing.Verify(tampered, controller) == nil {
			t.Fatalf("accepted tampered %s", field)
		}
	}
	unauthorized := controller
	unauthorized.AssertionMethod = []string{}
	if !errors.Is(signing.Verify(first, unauthorized), signing.ErrUntrustedMethod) {
		t.Fatal("accepted unauthorized key")
	}
	other, err := signing.NewLocalKey(k.ID(), k.Controller(), base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	wrongMethod, err := signing.PublicMultikey(other)
	if err != nil {
		t.Fatal(err)
	}
	wrongController, err := signing.NewControllerDocument(k.Controller(), []signing.Multikey{wrongMethod})
	if err != nil || signing.Verify(first, wrongController) == nil {
		t.Fatalf("accepted a different public key: %v", err)
	}
}
func TestRejectUnknownContextOrTerm(t *testing.T) {
	bad := strings.Replace(document, `"encodedList":`, `"unknownAcademyTerm":"secret","encodedList":`, 1)
	if _, err := signing.Sign(context.Background(), []byte(bad), time.Now(), key(t)); !errors.Is(err, signing.ErrUnsupportedContext) {
		t.Fatal(err)
	}
}

func TestOpenBadgeContextSigning(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	mapper, err := openbadges.NewMapper(openbadges.Config{PublicBaseURL: "https://academy.example", SubjectSalt: bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	cert := credentials.Certificate{ID: "11111111-1111-4111-8111-111111111111", LearnerUserID: "22222222-2222-4222-8222-222222222222", CourseID: "33333333-3333-4333-8333-333333333333", CourseVersionID: "44444444-4444-4444-8444-444444444444", Achievement: credentials.Achievement{CourseTitle: "Course", CourseVersion: "1.0.0", Language: "en", Criteria: "Complete every lesson."}, Issuer: credentials.Issuer{ID: "https://academy.example/issuer", Name: "Academy"}, IssuedAt: now, Status: credentials.CertificateActive}
	unsigned, err := mapper.Map(cert)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	signed, err := signing.Sign(context.Background(), raw, now, key(t))
	if err != nil {
		t.Fatal(err)
	}
	method, _ := signing.PublicMultikey(key(t))
	controller, _ := signing.NewControllerDocument(cert.Issuer.ID, []signing.Multikey{method})
	if err = signing.Verify(signed, controller); err != nil {
		t.Fatal(err)
	}
}

func TestOpenBadgeTamperingAndUntrustedAssertion(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	mapper, err := openbadges.NewMapper(openbadges.Config{PublicBaseURL: "https://academy.example", SubjectSalt: bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	cert := credentials.Certificate{ID: "11111111-1111-4111-8111-111111111111", LearnerUserID: "22222222-2222-4222-8222-222222222222", CourseID: "33333333-3333-4333-8333-333333333333", CourseVersionID: "44444444-4444-4444-8444-444444444444", Achievement: credentials.Achievement{CourseTitle: "Course", CourseVersion: "1.0.0", Language: "en", Criteria: "Complete every lesson."}, Issuer: credentials.Issuer{ID: "https://academy.example/issuer", Name: "Academy"}, IssuedAt: now, Status: credentials.CertificateActive}
	unsigned, err := mapper.Map(cert)
	if err != nil {
		t.Fatal(err)
	}
	unsigned.CredentialStatus = &openbadges.StatusReference{ID: "https://academy.example/status/1#entry", Type: "BitstringStatusListEntry", StatusPurpose: "revocation", StatusListIndex: "41", StatusListCredential: "https://academy.example/status/1"}
	raw, _ := json.Marshal(unsigned)
	signed, err := signing.Sign(context.Background(), raw, now, key(t))
	if err != nil {
		t.Fatal(err)
	}
	method, _ := signing.PublicMultikey(key(t))
	controller, _ := signing.NewControllerDocument(cert.Issuer.ID, []signing.Multikey{method})
	if err = signing.Verify(signed, controller); err != nil {
		t.Fatal(err)
	}
	variants := map[string]func(map[string]any){
		"title": func(doc map[string]any) {
			doc["credentialSubject"].(map[string]any)["achievement"].(map[string]any)["name"] = "Changed"
		},
		"version": func(doc map[string]any) {
			doc["credentialSubject"].(map[string]any)["achievement"].(map[string]any)["description"] = "Version 2.0.0"
		},
		"criteria": func(doc map[string]any) {
			doc["credentialSubject"].(map[string]any)["achievement"].(map[string]any)["criteria"].(map[string]any)["narrative"] = "Different"
		},
		"subject":         func(doc map[string]any) { doc["credentialSubject"].(map[string]any)["id"] = "urn:btg:subject:changed" },
		"issuer":          func(doc map[string]any) { doc["issuer"].(map[string]any)["name"] = "Different" },
		"issuedAt":        func(doc map[string]any) { doc["validFrom"] = "2026-09-24T12:00:00Z" },
		"id":              func(doc map[string]any) { doc["id"] = "https://academy.example/other" },
		"statusReference": func(doc map[string]any) { doc["credentialStatus"].(map[string]any)["statusListIndex"] = "42" },
		"proofValue":      func(doc map[string]any) { doc["proof"].(map[string]any)["proofValue"] = "z111111111" },
	}
	for name, mutate := range variants {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			if json.Unmarshal(signed, &doc) != nil {
				t.Fatal("invalid fixture")
			}
			mutate(doc)
			changed, _ := json.Marshal(doc)
			if signing.Verify(changed, controller) == nil {
				t.Fatal("tamper accepted")
			}
		})
	}
	unauthorized := controller
	unauthorized.AssertionMethod = nil
	if !errors.Is(signing.Verify(signed, unauthorized), signing.ErrUntrustedMethod) {
		t.Fatal("valid signature accepted without assertion authorization")
	}
}

func TestStatusListTampering(t *testing.T) {
	k := key(t)
	created := time.Date(2026, 9, 23, 12, 30, 0, 0, time.UTC)
	secured, err := signing.Sign(context.Background(), []byte(document), created, k)
	if err != nil {
		t.Fatal(err)
	}
	method, _ := signing.PublicMultikey(k)
	controller, _ := signing.NewControllerDocument(k.Controller(), []signing.Multikey{method})
	for name, change := range map[string]func(map[string]any){
		"encodedList": func(d map[string]any) { d["credentialSubject"].(map[string]any)["encodedList"] = "uH4sIAAAAAAAC" },
		"listID":      func(d map[string]any) { d["id"] = "https://academy.example/status/2" },
		"issuer":      func(d map[string]any) { d["issuer"].(map[string]any)["name"] = "Changed" },
		"purpose":     func(d map[string]any) { d["credentialSubject"].(map[string]any)["statusPurpose"] = "suspension" },
	} {
		t.Run(name, func(t *testing.T) {
			var d map[string]any
			if err := json.Unmarshal(secured, &d); err != nil {
				t.Fatal(err)
			}
			change(d)
			tampered, _ := json.Marshal(d)
			if signing.Verify(tampered, controller) == nil {
				t.Fatal("tamper accepted")
			}
		})
	}
}
