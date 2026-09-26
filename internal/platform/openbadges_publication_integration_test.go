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
	"os"
	"os/exec"
	"path/filepath"
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
	runDigitalBazaarInterop(t, ctx, badge, activeStatus, controller, false)
	if obj.CredentialStatus.StatusListIndex != strconv.Itoa(entry.StatusListIndex) || obj.CredentialStatus.StatusPurpose != "revocation" || obj.CredentialStatus.StatusListCredential != "https://academy.example/open-badges/status/revocation/"+entry.StatusListID {
		t.Fatal("credential status reference does not match its durable allocation")
	}
	var activeRevision int64
	if err = pool.QueryRow(ctx, `SELECT lifecycle_revision FROM open_badges.status_list WHERE id=$1`, entry.StatusListID).Scan(&activeRevision); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	// A failed orchestration does persist the revocation, but it returns an
	// explicit failure and leaves portable status unavailable until retry.
	failing, err := publication.NewService(certRepo, store, mapper, failingBadgeKey{key}, issuer, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if revoked, revokeErr := failing.Revoke(ctx, certRepo, cert.ID, issued.Add(2*time.Minute)); !errors.Is(revokeErr, publication.ErrRevocationPublicationFailed) || revoked.Status != credentials.CertificateRevoked {
		t.Fatalf("failed revocation=%#v,%v", revoked, revokeErr)
	}
	oldSnapshot, err := store.GetPublishedSnapshot(ctx, entry.StatusListID)
	if err != nil || !bytes.Equal(oldSnapshot.Document, activeStatus) {
		t.Fatalf("failed signing replaced previous snapshot: %v", err)
	}
	if _, err = failing.Status(ctx, entry.StatusListID); !errors.Is(err, publication.ErrUnavailable) {
		t.Fatalf("stale portable status was served after failed revocation: %v", err)
	}
	var revokedRevision int64
	if err = pool.QueryRow(ctx, `SELECT lifecycle_revision FROM open_badges.status_list WHERE id=$1`, entry.StatusListID).Scan(&revokedRevision); err != nil || revokedRevision != activeRevision+1 {
		t.Fatalf("revocation lifecycle revision=%d,%v want=%d", revokedRevision, err, activeRevision+1)
	}
	// Retry converges: Certificate Revoke is idempotent and only the missing
	// current signed snapshot is rebuilt.
	revoked, err := service.Revoke(ctx, certRepo, cert.ID, issued.Add(3*time.Minute))
	if err != nil || revoked.Status != credentials.CertificateRevoked {
		t.Fatalf("revocation retry=%#v,%v", revoked, err)
	}
	revokedStatus, err := service.Status(ctx, entry.StatusListID)
	if err != nil {
		t.Fatal(err)
	}
	var publishedRevision int64
	if err = pool.QueryRow(ctx, `SELECT lifecycle_revision FROM open_badges.status_list WHERE id=$1`, entry.StatusListID).Scan(&publishedRevision); err != nil || publishedRevision != revokedRevision {
		t.Fatalf("publication changed lifecycle revision=%d,%v want=%d", publishedRevision, err, revokedRevision)
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
	runDigitalBazaarInterop(t, ctx, badge, revokedStatus, controller, true)
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
	router.Get("/verify/certificates/{certificateId}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vc" {
			handler.handleSignedOpenBadge(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	badgeHTTP := get("/api/public/open-badges/" + string(cert.ID))
	if badgeHTTP.Code != http.StatusOK || badgeHTTP.Header().Get("Content-Type") != "application/vc" || badgeHTTP.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || !bytes.Equal(badgeHTTP.Body.Bytes(), badge) {
		t.Fatalf("badge HTTP: %d %s", badgeHTTP.Code, badgeHTTP.Body.String())
	}
	verificationRequest := httptest.NewRequest(http.MethodGet, "/verify/certificates/"+string(cert.ID), nil)
	verificationRequest.Header.Set("Accept", "application/vc")
	verificationHTTP := httptest.NewRecorder()
	router.ServeHTTP(verificationHTTP, verificationRequest)
	if verificationHTTP.Code != http.StatusOK || !bytes.Equal(verificationHTTP.Body.Bytes(), badge) {
		t.Fatalf("credential identifier did not resolve to badge: %d %s", verificationHTTP.Code, verificationHTTP.Body.String())
	}
	listHTTP := get("/open-badges/status/revocation/" + entry.StatusListID)
	if listHTTP.Code != http.StatusOK || listHTTP.Header().Get("Content-Type") != "application/vc" || listHTTP.Header().Get("Cache-Control") != "no-store" || !bytes.Equal(listHTTP.Body.Bytes(), revokedStatus) {
		t.Fatalf("list HTTP: %d %s", listHTTP.Code, listHTTP.Body.String())
	}
	issuerHTTP := get("/open-badges/issuer")
	if issuerHTTP.Code != http.StatusOK || issuerHTTP.Header().Get("Content-Type") != "application/json" || issuerHTTP.Header().Get("Cache-Control") != "public, max-age=300" || bytes.Contains(issuerHTTP.Body.Bytes(), seed) || !bytes.Contains(issuerHTTP.Body.Bytes(), []byte("assertionMethod")) {
		t.Fatalf("issuer HTTP: %d %s", issuerHTTP.Code, issuerHTTP.Body.String())
	}
	achievementHTTP := get("/achievements/course-versions/" + string(version.ID))
	if achievementHTTP.Code != http.StatusOK || achievementHTTP.Header().Get("Content-Type") != "application/json" || achievementHTTP.Header().Get("Cache-Control") != "public, max-age=86400" || !bytes.Contains(achievementHTTP.Body.Bytes(), []byte("1.0.0")) || bytes.Contains(achievementHTTP.Body.Bytes(), []byte(string(learner.ID))) {
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
	// Failure before the lifecycle transition is also visible and leaves the
	// Certificate ACTIVE, so an operator can retry without a partial revocation.
	otherLearner, err := identitypostgres.New(pool).CreateUser(ctx, identity.UserActive)
	if err != nil {
		t.Fatal(err)
	}
	otherInput := certificateInput(t, otherLearner.ID, version, credentials.CourseCompletionCriteriaText)
	otherInput.Issuer = issuer
	other, err := certRepo.Create(ctx, otherInput, issued)
	if err != nil {
		t.Fatal(err)
	}
	if result, revokeErr := failing.Revoke(ctx, certRepo, other.ID, issued.Add(4*time.Minute)); !errors.Is(revokeErr, publication.ErrRevocationPublicationFailed) || result.Status != credentials.CertificateActive {
		t.Fatalf("baseline publication failure=%#v,%v", result, revokeErr)
	}
	storedOther, err := certRepo.GetByID(ctx, other.ID)
	if err != nil || storedOther.Status != credentials.CertificateActive {
		t.Fatalf("baseline failure changed Certificate lifecycle=%#v,%v", storedOther, err)
	}
}

type failingBadgeKey struct{ signing.KeyProvider }

func (failingBadgeKey) Sign(context.Context, []byte) ([]byte, error) {
	return nil, errors.New("signer unavailable")
}

func runDigitalBazaarInterop(t *testing.T, ctx context.Context, badge, statusList []byte, issuer signing.ControllerDocument, revoked bool) {
	t.Helper()
	var badgeDocument, statusDocument any
	if err := json.Unmarshal(badge, &badgeDocument); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(statusList, &statusDocument); err != nil {
		t.Fatal(err)
	}
	fixture, err := json.Marshal(map[string]any{"badge": badgeDocument, "statusList": statusDocument, "issuer": issuer, "expected": map[string]bool{"revoked": revoked}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "openbadges-interop.json")
	if err = os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "tools", "openbadges-interop.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "node", script, path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Digital Bazaar interoperability failed: %v\n%s", err, output)
	}
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
