//go:build integration

package platform

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	coursespostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges"
	statuspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/publication"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/openbadges/signing"
	credentialspostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials/postgres"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity"
	identitypostgres "github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/identity/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testSignedOpenBadgesPublication(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	issuer := credentials.Issuer{ID: "https://academy.example/open-badges/issuer", Name: "Academy"}
	courseRepo := coursespostgres.New(pool)
	course, err := courseRepo.CreateCourse(ctx, "signed-badge-integration")
	if err != nil {
		t.Fatal(err)
	}
	version, err := courses.NewCourseVersionStore(courseRepo).Store(ctx, immutableCourseVersionFixture(t, course.ID, "1.0.0", "c9000000-0000-4000-8000-000000000002"))
	if err != nil {
		t.Fatal(err)
	}
	learner, err := identitypostgres.New(pool).CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	certRepo := credentialspostgres.New(pool)
	input := certificateInput(t, learner.ID, version, credentials.CourseCompletionCriteriaText)
	input.Issuer = issuer
	issued := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	cert, err := certRepo.Create(ctx, input, issued)
	if err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{9}, ed25519.SeedSize)
	key, err := signing.NewLocalKey(issuer.ID+"#key-1", issuer.ID, base64.RawURLEncoding.EncodeToString(seed))
	if err != nil {
		t.Fatal(err)
	}
	mapper, err := openbadges.NewMapper(openbadges.Config{PublicBaseURL: "https://academy.example", SubjectSalt: bytes.Repeat([]byte{3}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	store := statuspostgres.New(pool)
	now := issued.Add(time.Minute)
	service, err := publication.NewService(certRepo, store, mapper, key, issuer, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RegisterKey(ctx); err != nil {
		t.Fatal(err)
	}
	badge, err := service.Credential(ctx, cert.ID)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := service.Controller(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = signing.Verify(badge, controller); err != nil {
		t.Fatal(err)
	}
	var obj struct {
		CredentialStatus openbadges.StatusReference `json:"credentialStatus"`
	}
	if err = json.Unmarshal(badge, &obj); err != nil {
		t.Fatal(err)
	}
	entry, err := store.EnsureEntry(ctx, string(cert.ID))
	if err != nil {
		t.Fatal(err)
	}
	activeStatus, err := service.Status(ctx, entry.StatusListID)
	if err != nil {
		t.Fatal(err)
	}
	if err = signing.Verify(activeStatus, controller); err != nil {
		t.Fatal(err)
	}
	if statusBit(t, activeStatus, entry.StatusListIndex) {
		t.Fatal("active Certificate has revoked bit")
	}
	if obj.CredentialStatus.StatusListIndex != strconv.Itoa(entry.StatusListIndex) || obj.CredentialStatus.StatusPurpose != "revocation" || obj.CredentialStatus.StatusListCredential != "https://academy.example/open-badges/status/revocation/"+entry.StatusListID {
		t.Fatal("credential status reference does not match its durable allocation")
	}
	_, err = certRepo.Revoke(ctx, cert.ID, issued.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	// A failed signer must leave the previous complete snapshot intact, while
	// the public path refuses to serve its now-stale ACTIVE bit.
	failing, err := publication.NewService(certRepo, store, mapper, failingBadgeKey{key}, issuer, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = failing.Status(ctx, entry.StatusListID); !errors.Is(err, publication.ErrUnavailable) {
		t.Fatalf("failed signing returned %v", err)
	}
	oldSnapshot, err := store.GetPublishedSnapshot(ctx, entry.StatusListID)
	if err != nil || !bytes.Equal(oldSnapshot.Document, activeStatus) {
		t.Fatalf("failed signing replaced previous snapshot: %v", err)
	}
	revokedStatus, err := service.Status(ctx, entry.StatusListID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(activeStatus, revokedStatus) {
		t.Fatal("revocation did not refresh signed list")
	}
	if err = signing.Verify(revokedStatus, controller); err != nil {
		t.Fatal(err)
	}
	if !statusBit(t, revokedStatus, entry.StatusListIndex) {
		t.Fatal("revoked Certificate bit absent")
	}
	stableBadge, err := service.Credential(ctx, cert.ID)
	if err != nil || !bytes.Equal(badge, stableBadge) {
		t.Fatalf("credential changed after revocation: %v", err)
	}
	handler := &authHTTP{badgePublication: service, certificates: certRepo, achievementVersions: courseRepo, badgePublicOrigin: "https://academy.example", badgeIssuer: issuer}
	router := chi.NewRouter()
	router.Get("/api/public/open-badges/{certificateId}", handler.handleSignedOpenBadge)
	router.Get("/open-badges/status/revocation/{listId}", handler.handleSignedStatusList)
	router.Get("/open-badges/issuer", handler.handleOpenBadgesIssuer)
	router.Get("/achievements/course-versions/{versionId}", handler.handleOpenBadgesAchievement)
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	badgeHTTP := get("/api/public/open-badges/" + string(cert.ID))
	if badgeHTTP.Code != http.StatusOK || badgeHTTP.Header().Get("Content-Type") != "application/vc" || !bytes.Equal(badgeHTTP.Body.Bytes(), badge) {
		t.Fatalf("badge HTTP: %d %s", badgeHTTP.Code, badgeHTTP.Body.String())
	}
	listHTTP := get("/open-badges/status/revocation/" + entry.StatusListID)
	if listHTTP.Code != http.StatusOK || listHTTP.Header().Get("Cache-Control") != "no-store" || !bytes.Equal(listHTTP.Body.Bytes(), revokedStatus) {
		t.Fatalf("list HTTP: %d %s", listHTTP.Code, listHTTP.Body.String())
	}
	issuerHTTP := get("/open-badges/issuer")
	if issuerHTTP.Code != http.StatusOK || bytes.Contains(issuerHTTP.Body.Bytes(), seed) || !bytes.Contains(issuerHTTP.Body.Bytes(), []byte("assertionMethod")) {
		t.Fatalf("issuer HTTP: %d %s", issuerHTTP.Code, issuerHTTP.Body.String())
	}
	achievementHTTP := get("/achievements/course-versions/" + string(version.ID))
	if achievementHTTP.Code != http.StatusOK || !bytes.Contains(achievementHTTP.Body.Bytes(), []byte("1.0.0")) || bytes.Contains(achievementHTTP.Body.Bytes(), []byte(string(learner.ID))) {
		t.Fatalf("achievement HTTP: %d %s", achievementHTTP.Code, achievementHTTP.Body.String())
	}
	// Registering a new active method retains the old assertion method, so
	// historical credentials remain verifiable after ordinary key rotation.
	nextKey, err := signing.NewLocalKey(issuer.ID+"#key-2", issuer.ID, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	reusedID, err := signing.NewLocalKey(issuer.ID+"#key-1", issuer.ID, base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{8}, ed25519.SeedSize)))
	if err != nil {
		t.Fatal(err)
	}
	conflictingMethod, err := signing.PublicMultikey(reusedID)
	if err != nil || store.RegisterMethod(ctx, conflictingMethod) == nil {
		t.Fatalf("accepted a different public key under historical key ID: %v", err)
	}
	rotated, err := publication.NewService(certRepo, store, mapper, nextKey, issuer, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if err = rotated.RegisterKey(ctx); err != nil {
		t.Fatal(err)
	}
	rotatedController, err := rotated.Controller(ctx)
	if err != nil || len(rotatedController.AssertionMethod) != 2 || signing.Verify(badge, rotatedController) != nil {
		t.Fatalf("historical method lost after rotation: %v", err)
	}
}

type failingBadgeKey struct{ signing.KeyProvider }

func (failingBadgeKey) Sign(context.Context, []byte) ([]byte, error) {
	return nil, errors.New("signer unavailable")
}
func statusBit(t *testing.T, document []byte, index int) bool {
	t.Helper()
	var value struct {
		CredentialSubject struct {
			EncodedList string `json:"encodedList"`
		} `json:"credentialSubject"`
	}
	if err := json.Unmarshal(document, &value); err != nil {
		t.Fatal(err)
	}
	encoded := value.CredentialSubject.EncodedList
	if len(encoded) < 2 || encoded[0] != 'u' {
		t.Fatal("invalid encodedList")
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
	_ = reader.Close()
	if index >= len(raw)*8 {
		t.Fatal("index out of range " + strconv.Itoa(index))
	}
	return raw[index/8]&(1<<(7-index%8)) != 0
}
