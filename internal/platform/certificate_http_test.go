package platform

import (
	"context"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/credentials"
	"net/http"
	"strings"
	"testing"
	"time"
)

const certificateTestCourse = "11111111-1111-4111-8111-111111111111"
const certificateTestID = "22222222-2222-4222-8222-222222222222"

type certificateServiceFake struct {
	result  credentials.IssuanceResult
	learner string
	err     error
}

func (f *certificateServiceFake) Issue(_ context.Context, learner string, _ courses.CourseID, _ courses.Version) (credentials.IssuanceResult, error) {
	f.learner = learner
	return f.result, f.err
}

type certificateRepoFake struct {
	c   credentials.Certificate
	err error
}

func (f certificateRepoFake) Create(context.Context, credentials.CertificateInput, time.Time) (credentials.Certificate, error) {
	return f.c, f.err
}
func (f certificateRepoFake) GetByID(context.Context, credentials.CertificateID) (credentials.Certificate, error) {
	return f.c, f.err
}
func (f certificateRepoFake) GetByLearnerAndCourseVersion(context.Context, string, string) (credentials.Certificate, error) {
	return f.c, f.err
}
func (f certificateRepoFake) Revoke(context.Context, credentials.CertificateID, time.Time) (credentials.Certificate, error) {
	return f.c, f.err
}
func certificateFixture(owner string) credentials.Certificate {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return credentials.Certificate{ID: certificateTestID, LearnerUserID: owner, CourseID: certificateTestCourse, CourseVersionID: "33333333-3333-4333-8333-333333333333", Achievement: credentials.Achievement{CourseTitle: "Course", CourseVersion: "1.0.0", Language: "en", Criteria: "Completed every published lesson in this CourseVersion."}, Issuer: credentials.Issuer{ID: "https://academy.example", Name: "Academy"}, IssuedAt: at, Status: credentials.CertificateActive}
}
func TestCertificateHTTPPrivateAndPublicProjections(t *testing.T) {
	owner := string(loginTestUser)
	c := certificateFixture(owner)
	service := &certificateServiceFake{result: credentials.IssuanceResult{Certificate: &c}}
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, certificateIssuance: service, certificates: certificateRepoFake{c: c}})
	issue := authRequest(router, http.MethodPost, "/api/courses/by-id/"+certificateTestCourse+"/versions/1.0.0/certificate", "", communityCookie(), authTestCSRFToken().Value())
	if issue.Code != http.StatusCreated || service.learner != owner || strings.Contains(issue.Body.String(), owner) || issue.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("issue=%d %s", issue.Code, issue.Body.String())
	}
	public := authRequest(router, http.MethodGet, "/api/public/certificates/"+certificateTestID, "", nil)
	if public.Code != http.StatusOK || strings.Contains(public.Body.String(), owner) || !strings.Contains(public.Body.String(), `"status":"ACTIVE"`) {
		t.Fatalf("public=%d %s", public.Code, public.Body.String())
	}
}
func TestCertificateHTTPOwnershipAndEligibility(t *testing.T) {
	owner := string(loginTestUser)
	c := certificateFixture(owner)
	router := authTestRouter(&authHTTP{sessions: &authResolverFake{current: loginTestCurrent(t)}, certificateIssuance: &certificateServiceFake{result: credentials.IssuanceResult{}}, certificates: certificateRepoFake{c: c}})
	notEligible := authRequest(router, http.MethodPost, "/api/courses/by-id/"+certificateTestCourse+"/versions/1.0.0/certificate", "", communityCookie(), authTestCSRFToken().Value())
	if notEligible.Code != http.StatusConflict || !strings.Contains(notEligible.Body.String(), "certificate_not_eligible") {
		t.Fatalf("eligibility=%d %s", notEligible.Code, notEligible.Body.String())
	}

}

var _ = errors.New
