package credentials

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress"
)

const (
	issuanceLearner = "50000000-0000-4000-8000-000000000001"
	issuanceCourse  = "60000000-0000-4000-8000-000000000001"
	issuanceV1      = "70000000-0000-4000-8000-000000000001"
	issuanceV2      = "70000000-0000-4000-8000-000000000002"
)

func issuanceVersion(id, version string, lessons ...string) courses.ImmutableCourseVersion {
	parsed, _ := courses.ParseVersion(version)
	module := courses.ImmutableCourseVersionModule{StableKey: "module", Lessons: make([]courses.ImmutableCourseVersionLesson, 0, len(lessons))}
	for index, key := range lessons {
		module.Lessons = append(module.Lessons, courses.ImmutableCourseVersionLesson{StableKey: key, Position: index})
	}
	return courses.ImmutableCourseVersion{ID: courses.CourseVersionID(id), CourseVersion: courses.CourseVersionInput{CourseID: courses.CourseID(issuanceCourse), Version: parsed, Status: courses.CourseVersionPublished, Title: "Certificate course", SourceLanguage: "en-GB"}, Modules: []courses.ImmutableCourseVersionModule{module}}
}

func completion(t *testing.T, learner, version string, keys ...string) progress.CourseProgress {
	t.Helper()
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	value, err := progress.New(learner, version, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		value, _, err = value.MarkLessonCompleted(key, now.Add(time.Second))
		if err != nil {
			t.Fatal(err)
		}
	}
	return value
}

func TestEvaluatePublishedCourseCompletion(t *testing.T) {
	version := issuanceVersion(issuanceV1, "1.0.0", "lesson-a", "lesson-b")
	for name, completion := range map[string]*progress.CourseProgress{
		"missing":         nil,
		"incomplete":      pointer(completion(t, issuanceLearner, issuanceV1, "lesson-a")),
		"foreign version": pointer(completion(t, issuanceLearner, issuanceV2, "lesson-a", "lesson-b")),
		"eligible":        pointer(completion(t, issuanceLearner, issuanceV1, "lesson-a", "lesson-b")),
	} {
		t.Run(name, func(t *testing.T) {
			decision, err := EvaluatePublishedCourseCompletion(version, completion)
			if err != nil {
				t.Fatal(err)
			}
			if name == "eligible" {
				if decision != (Eligibility{Status: EligibilityEligible}) {
					t.Fatalf("decision = %#v", decision)
				}
				return
			}
			if decision.Status != EligibilityNotEligible {
				t.Fatalf("decision = %#v", decision)
			}
		})
	}
	empty, err := EvaluatePublishedCourseCompletion(issuanceVersion(issuanceV1, "1.0.0"), nil)
	if err != nil || empty != (Eligibility{Status: EligibilityNotEligible, Reason: EligibilityNoPublishedLessons}) {
		t.Fatalf("empty CourseVersion = %#v, %v", empty, err)
	}
	malformed := issuanceVersion(issuanceV1, "1.0.0", "lesson-a", "lesson-a")
	if _, err := EvaluatePublishedCourseCompletion(malformed, nil); !errors.Is(err, ErrEligibilityUnavailable) {
		t.Fatalf("malformed requirements = %v", err)
	}
}

func TestCertificateIssuanceUsesExactProgressAndExistingCertificateRecovery(t *testing.T) {
	now := time.Date(2026, time.September, 23, 12, 0, 0, 0, time.UTC)
	v1, v2 := issuanceVersion(issuanceV1, "1.0.0", "lesson-a"), issuanceVersion(issuanceV2, "1.1.0", "lesson-b")
	certificates := &issuanceCertificateRepository{items: map[string]Certificate{}}
	progresses := &issuanceProgressRepository{items: map[string]progress.CourseProgress{progressKey(issuanceLearner, issuanceV1): completion(t, issuanceLearner, issuanceV1, "lesson-a")}}
	service, err := NewCertificateIssuanceService(certificates, issuanceCourses{versions: map[string]courses.ImmutableCourseVersion{courseVersionKey(issuanceCourse, "1.0.0"): v1, courseVersionKey(issuanceCourse, "1.1.0"): v2}}, progresses, Issuer{ID: "https://academy.example.com", Name: "Academy"}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	issued, err := service.Issue(context.Background(), issuanceLearner, issuanceCourse, mustVersion(t, "1.0.0"))
	if err != nil || issued.Certificate == nil || issued.Existing || issued.Eligibility.Status != EligibilityEligible {
		t.Fatalf("issue = %#v, %v", issued, err)
	}
	if issued.Certificate.CourseVersionID != issuanceV1 || issued.Certificate.Issuer.Name != "Academy" || issued.Certificate.Achievement.Criteria != courseCompletionCriteriaText || !issued.Certificate.IssuedAt.Equal(now) {
		t.Fatalf("issued certificate = %#v", issued.Certificate)
	}
	progresses.items[progressKey(issuanceLearner, issuanceV2)] = completion(t, "50000000-0000-4000-8000-000000000002", issuanceV2, "lesson-b")
	notEligible, err := service.Issue(context.Background(), issuanceLearner, issuanceCourse, mustVersion(t, "1.1.0"))
	if err != nil || notEligible.Certificate != nil || notEligible.Eligibility != (Eligibility{Status: EligibilityNotEligible, Reason: EligibilityIncompleteLessons}) {
		t.Fatalf("v2 issuance = %#v, %v", notEligible, err)
	}

	progresses.items = map[string]progress.CourseProgress{}
	replayed, err := service.Issue(context.Background(), issuanceLearner, issuanceCourse, mustVersion(t, "1.0.0"))
	if err != nil || replayed.Certificate == nil || !replayed.Existing || replayed.Certificate.ID != issued.Certificate.ID {
		t.Fatalf("existing certificate recovery = %#v, %v", replayed, err)
	}
	issued.Certificate.Status = CertificateRevoked
	revokedAt := now.Add(time.Hour)
	issued.Certificate.RevokedAt = &revokedAt
	certificates.items[progressKey(issuanceLearner, issuanceV1)] = *issued.Certificate
	revoked, err := service.Issue(context.Background(), issuanceLearner, issuanceCourse, mustVersion(t, "1.0.0"))
	if err != nil || revoked.Certificate == nil || revoked.Certificate.Status != CertificateRevoked || !revoked.Existing {
		t.Fatalf("revoked recovery = %#v, %v", revoked, err)
	}
}

type issuanceCourses struct {
	versions map[string]courses.ImmutableCourseVersion
}

func (r issuanceCourses) GetImmutableCourseVersionByCourseAndVersion(_ context.Context, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	value, ok := r.versions[courseVersionKey(string(courseID), version.String())]
	if !ok {
		return courses.ImmutableCourseVersion{}, courses.ErrNotFound
	}
	return value, nil
}

type issuanceProgressRepository struct {
	items map[string]progress.CourseProgress
}

func (r *issuanceProgressRepository) Create(context.Context, string, string) (progress.CourseProgress, error) {
	return progress.CourseProgress{}, errors.New("unused")
}
func (r *issuanceProgressRepository) Get(_ context.Context, learner, version string) (progress.CourseProgress, error) {
	value, ok := r.items[progressKey(learner, version)]
	if !ok {
		return progress.CourseProgress{}, progress.ErrProgressNotFound
	}
	return value, nil
}
func (r *issuanceProgressRepository) MarkLessonCompleted(context.Context, string, string, string, int64) (progress.CourseProgress, error) {
	return progress.CourseProgress{}, errors.New("unused")
}

type issuanceCertificateRepository struct {
	mu    sync.Mutex
	items map[string]Certificate
}

func (r *issuanceCertificateRepository) Create(_ context.Context, input CertificateInput, at time.Time) (Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := progressKey(input.LearnerUserID, input.CourseVersionID)
	if existing, ok := r.items[key]; ok {
		return existing, nil
	}
	certificate := Certificate{ID: "80000000-0000-4000-8000-000000000001", LearnerUserID: input.LearnerUserID, CourseID: input.CourseID, CourseVersionID: input.CourseVersionID, Achievement: input.Achievement, Issuer: input.Issuer, IssuedAt: at.UTC(), Status: CertificateActive}
	r.items[key] = certificate
	return certificate, nil
}
func (r *issuanceCertificateRepository) GetByLearnerAndCourseVersion(_ context.Context, learner, version string) (Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.items[progressKey(learner, version)]
	if !ok {
		return Certificate{}, ErrCertificateNotFound
	}
	return value, nil
}

func mustVersion(t *testing.T, value string) courses.Version {
	t.Helper()
	version, err := courses.ParseVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
func pointer(value progress.CourseProgress) *progress.CourseProgress { return &value }
func progressKey(learner, version string) string                     { return learner + "/" + version }
func courseVersionKey(course, version string) string                 { return course + "/" + version }
