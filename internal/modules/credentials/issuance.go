package credentials

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/progress"
)

var ErrEligibilityUnavailable = errors.New("certificate eligibility is unavailable")

type EligibilityStatus string
type EligibilityReason string

const (
	EligibilityEligible    EligibilityStatus = "ELIGIBLE"
	EligibilityNotEligible EligibilityStatus = "NOT_ELIGIBLE"

	EligibilityCourseNotPublished EligibilityReason = "COURSE_NOT_PUBLISHED"
	EligibilityNoPublishedLessons EligibilityReason = "NO_PUBLISHED_LESSONS"
	EligibilityMissingProgress    EligibilityReason = "MISSING_PROGRESS"
	EligibilityIncompleteLessons  EligibilityReason = "INCOMPLETE_LESSONS"
)

// Eligibility is an internal, deterministic decision. Reasons are stable
// policy identifiers rather than transport-facing error strings.
type Eligibility struct {
	Status EligibilityStatus
	Reason EligibilityReason
}

// IssuanceResult keeps ineligible outcomes distinct from infrastructure
// failures. Existing is true when an earlier ACTIVE or REVOKED certificate is
// returned without re-evaluating current completion policy.
type IssuanceResult struct {
	Eligibility Eligibility
	Certificate *Certificate
	Existing    bool
}

// CourseVersionLookup is intentionally exact. It never selects a latest
// version and never reaches into Authoring.
type CourseVersionLookup interface {
	GetImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}

// CertificateIssuanceService coordinates private completion facts with an
// immutable published CourseVersion. It is transport-independent: a later
// HTTP boundary supplies the authenticated learner identity.
type CertificateIssuanceService struct {
	certificates CertificateRepository
	courses      CourseVersionLookup
	progress     progress.Repository
	issuer       Issuer
	now          func() time.Time
}

// CertificateRepository names the small part of persistence used by issuance.
// It remains separate from completion policy and Course reads.
type CertificateRepository interface {
	Create(context.Context, CertificateInput, time.Time) (Certificate, error)
	GetByLearnerAndCourseVersion(context.Context, string, string) (Certificate, error)
}

func NewCertificateIssuanceService(certificates CertificateRepository, courseVersions CourseVersionLookup, progressRepository progress.Repository, issuer Issuer, now func() time.Time) (*CertificateIssuanceService, error) {
	if certificates == nil || courseVersions == nil || progressRepository == nil || issuer.Validate() != nil {
		return nil, ErrEligibilityUnavailable
	}
	if now == nil {
		now = time.Now
	}
	return &CertificateIssuanceService{certificates: certificates, courses: courseVersions, progress: progressRepository, issuer: issuer, now: now}, nil
}

func (s *CertificateIssuanceService) Eligibility(ctx context.Context, learnerID string, courseID courses.CourseID, version courses.Version) (Eligibility, error) {
	if !validUUID(learnerID) {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	immutable, err := s.exactCourseVersion(ctx, courseID, version)
	if err != nil {
		return Eligibility{}, err
	}
	return s.evaluate(ctx, learnerID, immutable)
}

func (s *CertificateIssuanceService) Issue(ctx context.Context, learnerID string, courseID courses.CourseID, version courses.Version) (IssuanceResult, error) {
	if !validUUID(learnerID) {
		return IssuanceResult{}, ErrEligibilityUnavailable
	}
	immutable, err := s.exactCourseVersion(ctx, courseID, version)
	if err != nil {
		return IssuanceResult{}, err
	}
	existing, err := s.certificates.GetByLearnerAndCourseVersion(ctx, learnerID, string(immutable.ID))
	if err == nil {
		return IssuanceResult{Eligibility: Eligibility{Status: EligibilityEligible}, Certificate: &existing, Existing: true}, nil
	}
	if !errors.Is(err, ErrCertificateNotFound) {
		return IssuanceResult{}, err
	}
	eligibility, err := s.evaluate(ctx, learnerID, immutable)
	if err != nil {
		return IssuanceResult{}, err
	}
	if eligibility.Status != EligibilityEligible {
		return IssuanceResult{Eligibility: eligibility}, nil
	}
	achievement, err := CourseCompletionAchievement(immutable, courseCompletionCriteriaText)
	if err != nil {
		return IssuanceResult{}, ErrEligibilityUnavailable
	}
	certificate, err := s.certificates.Create(ctx, CertificateInput{
		LearnerUserID: learnerID, CourseID: string(immutable.CourseVersion.CourseID), CourseVersionID: string(immutable.ID), Achievement: achievement, Issuer: s.issuer,
	}, s.now().UTC())
	if err != nil {
		return IssuanceResult{}, err
	}
	return IssuanceResult{Eligibility: Eligibility{Status: EligibilityEligible}, Certificate: &certificate}, nil
}

const courseCompletionCriteriaText = "Completed every published lesson in this CourseVersion."

func (s *CertificateIssuanceService) exactCourseVersion(ctx context.Context, courseID courses.CourseID, version courses.Version) (courses.ImmutableCourseVersion, error) {
	if s == nil || s.courses == nil || !validUUID(string(courseID)) || !version.Valid() {
		return courses.ImmutableCourseVersion{}, ErrEligibilityUnavailable
	}
	immutable, err := s.courses.GetImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
	if err != nil {
		return courses.ImmutableCourseVersion{}, err
	}
	if immutable.ID == "" || immutable.CourseVersion.CourseID != courseID || immutable.CourseVersion.Version != version {
		return courses.ImmutableCourseVersion{}, ErrEligibilityUnavailable
	}
	return immutable, nil
}

func (s *CertificateIssuanceService) evaluate(ctx context.Context, learnerID string, immutable courses.ImmutableCourseVersion) (Eligibility, error) {
	if immutable.CourseVersion.Status != courses.CourseVersionPublished {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityCourseNotPublished}, nil
	}
	requirements, err := publishedLessonKeys(immutable)
	if err != nil {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	if len(requirements) == 0 {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityNoPublishedLessons}, nil
	}
	completion, err := s.progress.Get(ctx, learnerID, string(immutable.ID))
	if errors.Is(err, progress.ErrProgressNotFound) {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityMissingProgress}, nil
	}
	if err != nil {
		return Eligibility{}, err
	}
	if completion.LearnerUserID != learnerID || completion.CourseVersionID != string(immutable.ID) || completion.Validate() != nil || !progress.IsCourseComplete(requirements, completion) {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityIncompleteLessons}, nil
	}
	return Eligibility{Status: EligibilityEligible}, nil
}

// EvaluatePublishedCourseCompletion is a pure form of the v1 policy for
// focused tests and future read models. Nil progress means no durable progress
// record exists for the learner/version.
func EvaluatePublishedCourseCompletion(immutable courses.ImmutableCourseVersion, completion *progress.CourseProgress) (Eligibility, error) {
	if immutable.ID == "" || immutable.CourseVersion.Status != courses.CourseVersionPublished {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityCourseNotPublished}, nil
	}
	requirements, err := publishedLessonKeys(immutable)
	if err != nil {
		return Eligibility{}, ErrEligibilityUnavailable
	}
	if len(requirements) == 0 {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityNoPublishedLessons}, nil
	}
	if completion == nil {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityMissingProgress}, nil
	}
	if completion.CourseVersionID != string(immutable.ID) || completion.Validate() != nil || !progress.IsCourseComplete(requirements, *completion) {
		return Eligibility{Status: EligibilityNotEligible, Reason: EligibilityIncompleteLessons}, nil
	}
	return Eligibility{Status: EligibilityEligible}, nil
}

func publishedLessonKeys(immutable courses.ImmutableCourseVersion) ([]string, error) {
	keys := make([]string, 0)
	seen := make(map[string]struct{})
	for _, module := range immutable.Modules {
		for _, lesson := range module.Lessons {
			if lesson.StableKey == "" {
				return nil, ErrEligibilityUnavailable
			}
			if _, duplicate := seen[lesson.StableKey]; duplicate {
				return nil, ErrEligibilityUnavailable
			}
			seen[lesson.StableKey] = struct{}{}
			keys = append(keys, lesson.StableKey)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
