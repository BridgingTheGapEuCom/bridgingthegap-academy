package portability

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
)

// ErrorCode is deliberately stable for the future upload boundary. Details are
// safe internal locations, never raw ZIP or JSON parser errors.
type ErrorCode string

const (
	ErrInvalidArchive            ErrorCode = "invalid_archive"
	ErrPackageTooLarge           ErrorCode = "package_too_large"
	ErrUnsafeArchivePath         ErrorCode = "unsafe_archive_path"
	ErrMissingRequiredEntry      ErrorCode = "missing_required_entry"
	ErrUnexpectedEntry           ErrorCode = "unexpected_entry"
	ErrInvalidManifest           ErrorCode = "invalid_manifest"
	ErrUnsupportedPackageFormat  ErrorCode = "unsupported_package_format"
	ErrUnsupportedPackageVersion ErrorCode = "unsupported_package_version"
	ErrChecksumMismatch          ErrorCode = "checksum_mismatch"
	ErrInvalidCourse             ErrorCode = "invalid_course"
	ErrInvalidAssessment         ErrorCode = "invalid_assessment"
	ErrInvalidAsset              ErrorCode = "invalid_asset"
	ErrInvalidTranslation        ErrorCode = "invalid_translation"
)

type PackageError struct {
	Code     ErrorCode
	Location string
}

func (e *PackageError) Error() string {
	if e.Location == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Location
}
func packageError(code ErrorCode, location string) error {
	return &PackageError{Code: code, Location: location}
}
func IsPackageError(err error, code ErrorCode) bool {
	var e *PackageError
	return errors.As(err, &e) && e.Code == code
}

// Limits bound hostile package work. Zero-valued fields receive v1 defaults.
type Limits struct {
	MaxCompressedBytes   int64
	MaxUncompressedBytes int64
	MaxEntryBytes        int64
	MaxEntries           int
	MaxAssets            int
	MaxTranslations      int
	MaxCompressionRatio  int64
}

func DefaultLimits() Limits {
	return Limits{MaxCompressedBytes: 512 << 20, MaxUncompressedBytes: 1024 << 20, MaxEntryBytes: 256 << 20, MaxEntries: 4096, MaxAssets: 1000, MaxTranslations: 100, MaxCompressionRatio: 200}
}
func (l Limits) normalized() Limits {
	d := DefaultLimits()
	if l.MaxCompressedBytes <= 0 {
		l.MaxCompressedBytes = d.MaxCompressedBytes
	}
	if l.MaxUncompressedBytes <= 0 {
		l.MaxUncompressedBytes = d.MaxUncompressedBytes
	}
	if l.MaxEntryBytes <= 0 {
		l.MaxEntryBytes = d.MaxEntryBytes
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = d.MaxEntries
	}
	if l.MaxAssets <= 0 {
		l.MaxAssets = d.MaxAssets
	}
	if l.MaxTranslations <= 0 {
		l.MaxTranslations = d.MaxTranslations
	}
	if l.MaxCompressionRatio <= 0 {
		l.MaxCompressionRatio = d.MaxCompressionRatio
	}
	return l
}

type Reader struct{ limits Limits }

func NewReader(limits Limits) *Reader { return &Reader{limits: limits.normalized()} }

// Read consumes at most the configured compressed input, uses a temporary ZIP
// file only to satisfy archive/zip ReaderAt, and removes it before returning.
func (r *Reader) Read(ctx context.Context, input io.Reader) (*ValidatedCoursePackage, error) {
	if r == nil || input == nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	l := r.limits.normalized()
	tmp, err := os.CreateTemp("", "btg-course-package-*.zip")
	if err != nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	n, err := copyLimited(tmp, input, l.MaxCompressedBytes)
	if closeErr := tmp.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, packageError(ErrPackageTooLarge, "")
	}
	defer func() { _ = n }()
	file, err := os.Open(name)
	if err != nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	defer func() { _ = file.Close() }()
	return r.readAt(ctx, file, n)
}
func (r *Reader) readAt(ctx context.Context, file *os.File, size int64) (*ValidatedCoursePackage, error) {
	z, err := zip.NewReader(file, size)
	if err != nil {
		return nil, packageError(ErrInvalidArchive, "")
	}
	return r.validate(ctx, z)
}

// ValidatedCoursePackage has no public literal constructor. Future persistence
// must receive this value rather than raw archive DTOs.
type ValidatedCoursePackage struct {
	manifest     Manifest
	course       CoursePayload
	assessments  AssessmentsPayload
	translations []TranslationPayload
	assets       []validatedAsset
	digest       string
}
type validatedAsset struct {
	entry AssetEntry
	bytes []byte
}
type ImportPreview struct {
	Format               string
	FormatVersion        int
	Title                string
	Version              string
	Language             string
	License              License
	Attribution          []Attribution
	ModuleCount          int
	LessonCount          int
	AssessmentCount      int
	AssetCount           int
	AssetBytes           int64
	TranslationLanguages []string
	PackageDigest        string
}

func (p *ValidatedCoursePackage) Preview() ImportPreview {
	if p == nil {
		return ImportPreview{}
	}
	lessons := 0
	for _, m := range p.course.Modules {
		lessons += len(m.Lessons)
	}
	languages := make([]string, 0, len(p.translations))
	for _, t := range p.translations {
		languages = append(languages, t.Language)
	}
	sort.Strings(languages)
	total := int64(0)
	for _, a := range p.assets {
		total += a.entry.ByteSize
	}
	return ImportPreview{Format: p.manifest.Format, FormatVersion: p.manifest.FormatVersion, Title: p.course.Title, Version: p.course.Version, Language: p.course.Language, License: p.manifest.Course.License, Attribution: append([]Attribution(nil), p.manifest.Course.Attribution...), ModuleCount: len(p.course.Modules), LessonCount: lessons, AssessmentCount: len(p.assessments.Assessments), AssetCount: len(p.assets), AssetBytes: total, TranslationLanguages: languages, PackageDigest: p.digest}
}

func (r *Reader) validate(ctx context.Context, z *zip.Reader) (*ValidatedCoursePackage, error) {
	l := r.limits.normalized()
	if len(z.File) > l.MaxEntries {
		return nil, packageError(ErrPackageTooLarge, "entries")
	}
	raw := map[string][]byte{}
	total := int64(0)
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, packageError(ErrInvalidArchive, "")
		}
		name, err := safeArchivePath(f.Name)
		if err != nil {
			return nil, err
		}
		if _, ok := raw[name]; ok {
			return nil, packageError(ErrInvalidArchive, name)
		}
		if !regularZipFile(f) {
			return nil, packageError(ErrInvalidArchive, name)
		}
		if f.UncompressedSize64 > uint64(l.MaxEntryBytes) || f.CompressedSize64 > 0 && f.UncompressedSize64 > f.CompressedSize64*uint64(l.MaxCompressionRatio) {
			return nil, packageError(ErrPackageTooLarge, name)
		}
		b, err := readZipEntry(f, l.MaxEntryBytes)
		if err != nil {
			return nil, err
		}
		total += int64(len(b))
		if total > l.MaxUncompressedBytes {
			return nil, packageError(ErrPackageTooLarge, "uncompressed")
		}
		raw[name] = b
	}
	for _, p := range []string{manifestPath, coursePath, assessmentsPath, checksumsPath} {
		if _, ok := raw[p]; !ok {
			return nil, packageError(ErrMissingRequiredEntry, p)
		}
	}
	var manifest Manifest
	if err := strictDecode(raw[manifestPath], &manifest); err != nil {
		return nil, packageError(ErrInvalidManifest, manifestPath)
	}
	if manifest.Format != Format {
		return nil, packageError(ErrUnsupportedPackageFormat, "")
	}
	if manifest.FormatVersion != FormatVersion {
		return nil, packageError(ErrUnsupportedPackageVersion, "")
	}
	if manifest.ChecksumsPath != checksumsPath {
		return nil, packageError(ErrInvalidManifest, "checksumsPath")
	}
	var checks Checksums
	if err := strictDecode(raw[checksumsPath], &checks); err != nil {
		return nil, packageError(ErrInvalidManifest, checksumsPath)
	}
	if err := validateInventory(raw, manifest, checks, l); err != nil {
		return nil, err
	}
	var course CoursePayload
	if err := strictDecode(raw[coursePath], &course); err != nil {
		return nil, packageError(ErrInvalidCourse, coursePath)
	}
	var assessments AssessmentsPayload
	if err := strictDecode(raw[assessmentsPath], &assessments); err != nil {
		return nil, packageError(ErrInvalidAssessment, assessmentsPath)
	}
	if err := validateCourse(manifest, course, assessments); err != nil {
		return nil, err
	}
	source, err := sourceFor(manifest, course, assessments)
	if err != nil {
		return nil, err
	}
	translationsPayload := make([]TranslationPayload, 0, len(manifest.Translations))
	seenLang := map[string]bool{}
	for _, entry := range manifest.Translations {
		body := raw[entry.Path]
		var t TranslationPayload
		if err := strictDecode(body, &t); err != nil {
			return nil, packageError(ErrInvalidTranslation, entry.Path)
		}
		if t.Language != entry.Language || t.SourceVersion != entry.SourceVersion || t.SourceCourseID != manifest.Course.OriginCourseID || t.SourceCourseVersionID != manifest.Course.OriginCourseVersionID || seenLang[t.Language] || t.Language == course.Language {
			return nil, packageError(ErrInvalidTranslation, entry.Path)
		}
		lang, err := courses.NormalizeLanguageTag(t.Language)
		if err != nil || string(lang) != t.Language || t.Tree.ValidateAgainstSource(source) != nil || !translationComplete(t.Tree, source) {
			return nil, packageError(ErrInvalidTranslation, entry.Path)
		}
		seenLang[t.Language] = true
		translationsPayload = append(translationsPayload, t)
	}
	assets := make([]validatedAsset, 0, len(manifest.Assets))
	for _, entry := range manifest.Assets {
		b := raw[entry.Path]
		if int64(len(b)) != entry.ByteSize || digestBytes(b) != entry.SHA256 {
			return nil, packageError(ErrInvalidAsset, entry.Path)
		}
		assets = append(assets, validatedAsset{entry: entry, bytes: b})
	}
	// checksums.json is the canonical, sorted v1 immutable-content identity. It excludes manifest exportedAt and ZIP metadata.
	sum := sha256.Sum256(raw[checksumsPath])
	return &ValidatedCoursePackage{manifest: manifest, course: course, assessments: assessments, translations: translationsPayload, assets: assets, digest: hex.EncodeToString(sum[:])}, nil
}
func safeArchivePath(name string) (string, error) {
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.ContainsRune(name, '\x00') || path.Clean(name) != name || strings.HasPrefix(name, "../") || name == ".." {
		return "", packageError(ErrUnsafeArchivePath, "")
	}
	return name, nil
}
func regularZipFile(f *zip.File) bool {
	mode := f.Mode()
	return !f.FileInfo().IsDir() && mode&os.ModeType == 0
}
func readZipEntry(f *zip.File, max int64) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, packageError(ErrInvalidArchive, f.Name)
	}
	defer func() { _ = r.Close() }()
	b, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil || int64(len(b)) > max {
		return nil, packageError(ErrPackageTooLarge, f.Name)
	}
	return b, nil
}
func copyLimited(dst io.Writer, src io.Reader, max int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, max+1))
	if err != nil {
		return n, err
	}
	if n > max {
		return n, errors.New("limit")
	}
	return n, nil
}
func strictDecode(b []byte, v any) error {
	if !json.Valid(b) || duplicateJSONKey(b) {
		return errors.New("invalid json")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	var x any
	if err := d.Decode(&x); err != io.EOF {
		return errors.New("trailing json")
	}
	return nil
}
func duplicateJSONKey(b []byte) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	var walk func() bool
	walk = func() bool {
		t, e := d.Token()
		if e != nil {
			return true
		}
		if delim, ok := t.(json.Delim); ok && delim == '{' {
			seen := map[string]bool{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return true
				}
				s, ok := k.(string)
				if !ok || seen[s] {
					return true
				}
				seen[s] = true
				if walk() {
					return true
				}
			}
			_, e = d.Token()
			return e != nil
		}
		if delim, ok := t.(json.Delim); ok && delim == '[' {
			for d.More() {
				if walk() {
					return true
				}
			}
			_, e = d.Token()
			return e != nil
		}
		return false
	}
	return walk()
}
func validateInventory(raw map[string][]byte, m Manifest, c Checksums, l Limits) error {
	if len(m.Assets) > l.MaxAssets || len(m.Translations) > l.MaxTranslations {
		return packageError(ErrPackageTooLarge, "inventory")
	}
	declared := map[string]bool{manifestPath: true, checksumsPath: true, coursePath: true, assessmentsPath: true}
	checks := map[string]string{}
	for _, e := range c.Entries {
		if _, ok := checks[e.Path]; ok || len(e.SHA256) != 64 {
			return packageError(ErrChecksumMismatch, e.Path)
		}
		checks[e.Path] = e.SHA256
	}
	for _, e := range m.Contents {
		if e.Path == manifestPath || e.Path == checksumsPath || checks[e.Path] != e.SHA256 || digestBytes(raw[e.Path]) != e.SHA256 {
			return packageError(ErrChecksumMismatch, e.Path)
		}
		declared[e.Path] = true
	}
	assets := map[string]bool{}
	for _, a := range m.Assets {
		if _, ok := assets[a.AssetKey]; ok || a.Path != path.Join("assets", a.AssetKey, "content") || checks[a.Path] != a.SHA256 || digestBytes(raw[a.Path]) != a.SHA256 {
			return packageError(ErrInvalidAsset, a.Path)
		}
		assets[a.AssetKey] = true
		declared[a.Path] = true
	}
	translations := map[string]bool{}
	for _, t := range m.Translations {
		if translations[t.Language] || t.Path != path.Join("translations", t.Language+".json") || checks[t.Path] == "" || digestBytes(raw[t.Path]) != checks[t.Path] {
			return packageError(ErrInvalidTranslation, t.Path)
		}
		translations[t.Language] = true
		declared[t.Path] = true
	}
	for p := range raw {
		if !declared[p] {
			return packageError(ErrUnexpectedEntry, p)
		}
	}
	for p := range checks {
		if !declared[p] {
			return packageError(ErrChecksumMismatch, p)
		}
	}
	return nil
}
func validateCourse(m Manifest, c CoursePayload, a AssessmentsPayload) error {
	version, err := courses.ParseVersion(c.Version)
	if err != nil || c.Version != m.Course.Version || c.Language != m.Course.Language {
		return packageError(ErrInvalidCourse, "metadata")
	}
	license := courses.ContentLicense{Kind: courses.ContentLicenseKind(m.Course.License.Kind), Identifier: m.Course.License.Identifier, DisplayName: m.Course.License.DisplayName, URL: m.Course.License.URL, CustomText: m.Course.License.CustomText}
	contributors := make([]courses.ContributorSnapshot, 0, len(m.Course.Attribution))
	for _, x := range m.Course.Attribution {
		contributors = append(contributors, courses.ContributorSnapshot{DisplayName: x.DisplayName, Role: courses.ContributorRole(x.Role), Order: x.Order})
	}
	if (courses.CourseVersionInput{CourseID: "portable", Version: version, Status: courses.CourseVersionPublished, Title: c.Title, Description: c.Description, LearningObjectives: c.LearningObjectives, SourceLanguage: courses.LanguageTag(c.Language), Changelog: c.Changelog, License: license, Attribution: contributors, PublishedAt: time.Unix(1, 0)}).Validate() != nil {
		return packageError(ErrInvalidCourse, "metadata")
	}
	mods := map[string]bool{}
	lessons := map[string]bool{}
	for i, m := range c.Modules {
		if mods[m.StableKey] || m.Position != i || (courses.ModuleInput{CourseVersionID: "portable", StableKey: m.StableKey, Title: m.Title, Description: m.Description, Position: m.Position}).Validate() != nil {
			return packageError(ErrInvalidCourse, "modules")
		}
		mods[m.StableKey] = true
		for j, l := range m.Lessons {
			if lessons[l.StableKey] || l.Position != j || l.Content.Validate() != nil || (courses.LessonInput{CourseVersionID: "portable", ModuleID: "portable", StableKey: l.StableKey, Title: l.Title, Description: l.Description, LearningObjectives: l.LearningObjectives, EstimatedDurationMinutes: l.EstimatedDurationMinutes, Position: l.Position, Content: l.Content}).Validate() != nil {
				return packageError(ErrInvalidCourse, "lessons")
			}
			lessons[l.StableKey] = true
		}
	}
	for _, m := range c.Modules {
		for _, l := range m.Lessons {
			seen := map[string]bool{}
			for _, p := range l.PrerequisiteStableKeys {
				if p == l.StableKey || seen[p] || !lessons[p] {
					return packageError(ErrInvalidCourse, "prerequisites")
				}
				seen[p] = true
			}
		}
	}
	bindings := map[string]bool{}
	for _, x := range a.Assessments {
		b := courses.PublishedAssessmentBinding{AssessmentKey: x.AssessmentKey}
		for _, q := range x.Questions {
			z := courses.PublishedAssessmentQuestion{StableKey: q.StableKey, Type: q.Type, Prompt: q.Prompt, Position: q.Position, CorrectOptionKeys: q.CorrectOptionKeys}
			for _, o := range q.Options {
				z.Options = append(z.Options, courses.PublishedAssessmentOption(o))
			}
			for _, i := range q.LeftItems {
				z.LeftItems = append(z.LeftItems, courses.PublishedAssessmentItem(i))
			}
			for _, i := range q.RightItems {
				z.RightItems = append(z.RightItems, courses.PublishedAssessmentItem(i))
			}
			for _, p := range q.CorrectPairs {
				z.CorrectPairs = append(z.CorrectPairs, courses.PublishedAssessmentPair(p))
			}
			b.Questions = append(b.Questions, z)
		}
		if bindings[x.AssessmentKey] || b.Validate() != nil {
			return packageError(ErrInvalidAssessment, x.AssessmentKey)
		}
		bindings[x.AssessmentKey] = true
	}
	for _, m := range c.Modules {
		for _, l := range m.Lessons {
			for _, b := range l.Content.Blocks {
				if x, ok := b.Payload.(courses.KnowledgeCheckBlockPayload); ok && !bindings[x.AssessmentKey] {
					return packageError(ErrInvalidAssessment, x.AssessmentKey)
				}
			}
		}
	}
	assetKeys := map[string]bool{}
	for _, asset := range m.Assets {
		if assetKeys[asset.AssetKey] || asset.ByteSize <= 0 || len(asset.SHA256) != 64 || asset.OriginalFilename == "" || asset.MediaType == "" {
			return packageError(ErrInvalidAsset, asset.Path)
		}
		assetKeys[asset.AssetKey] = false
	}
	for _, module := range c.Modules {
		for _, lesson := range module.Lessons {
			for _, block := range lesson.Content.Blocks {
				var refs []string
				switch x := block.Payload.(type) {
				case courses.ImageBlockPayload:
					refs = []string{x.Asset.AssetKey}
				case courses.VideoBlockPayload:
					refs = []string{x.Asset.AssetKey, x.CaptionsAsset.AssetKey}
					if x.TranscriptAsset != nil {
						refs = append(refs, x.TranscriptAsset.AssetKey)
					}
				case courses.AudioBlockPayload:
					refs = []string{x.Asset.AssetKey}
					if x.TranscriptAsset != nil {
						refs = append(refs, x.TranscriptAsset.AssetKey)
					}
				case courses.DownloadBlockPayload:
					refs = []string{x.Asset.AssetKey}
				}
				for _, key := range refs {
					used, ok := assetKeys[key]
					if !ok {
						return packageError(ErrInvalidAsset, key)
					}
					assetKeys[key] = true
					_ = used
				}
			}
		}
	}
	for key, used := range assetKeys {
		if !used {
			return packageError(ErrInvalidAsset, key)
		}
	}
	return nil
}
func sourceFor(m Manifest, c CoursePayload, a AssessmentsPayload) (courses.ImmutableCourseVersion, error) {
	v, err := courses.ParseVersion(c.Version)
	if err != nil {
		return courses.ImmutableCourseVersion{}, packageError(ErrInvalidCourse, "version")
	}
	mods := make([]courses.ImmutableCourseVersionModule, 0, len(c.Modules))
	for _, m := range c.Modules {
		mm := courses.ImmutableCourseVersionModule{StableKey: m.StableKey, Title: m.Title, Description: m.Description, Position: m.Position}
		for _, l := range m.Lessons {
			mm.Lessons = append(mm.Lessons, courses.ImmutableCourseVersionLesson{StableKey: l.StableKey, Title: l.Title, Description: l.Description, LearningObjectives: l.LearningObjectives, EstimatedDurationMinutes: l.EstimatedDurationMinutes, Position: l.Position, PrerequisiteStableKeys: l.PrerequisiteStableKeys, Content: l.Content})
		}
		mods = append(mods, mm)
	}
	bindings := make([]courses.PublishedAssessmentBinding, 0, len(a.Assessments))
	for _, x := range a.Assessments {
		b := courses.PublishedAssessmentBinding{AssessmentKey: x.AssessmentKey}
		for _, q := range x.Questions {
			z := courses.PublishedAssessmentQuestion{StableKey: q.StableKey, Type: q.Type, Prompt: q.Prompt, Position: q.Position, CorrectOptionKeys: q.CorrectOptionKeys}
			for _, o := range q.Options {
				z.Options = append(z.Options, courses.PublishedAssessmentOption(o))
			}
			for _, i := range q.LeftItems {
				z.LeftItems = append(z.LeftItems, courses.PublishedAssessmentItem(i))
			}
			for _, i := range q.RightItems {
				z.RightItems = append(z.RightItems, courses.PublishedAssessmentItem(i))
			}
			for _, p := range q.CorrectPairs {
				z.CorrectPairs = append(z.CorrectPairs, courses.PublishedAssessmentPair(p))
			}
			b.Questions = append(b.Questions, z)
		}
		bindings = append(bindings, b)
	}
	return courses.ImmutableCourseVersion{ID: courses.CourseVersionID(m.Course.OriginCourseVersionID), CourseVersion: courses.CourseVersionInput{CourseID: courses.CourseID(m.Course.OriginCourseID), Version: v, Status: courses.CourseVersionPublished, Title: c.Title, Description: c.Description, LearningObjectives: c.LearningObjectives, SourceLanguage: courses.LanguageTag(c.Language)}, Modules: mods, AssessmentBindings: bindings}, nil
}
func translationComplete(t translations.TranslationTree, source courses.ImmutableCourseVersion) bool {
	required := func(values ...*string) bool {
		for _, value := range values {
			if value == nil {
				return false
			}
		}
		return true
	}
	if !required(t.Title, t.Description) {
		return false
	}
	for _, value := range t.LearningObjectives {
		if value == nil {
			return false
		}
	}
	for i, module := range source.Modules {
		tm := t.Modules[i]
		if !required(tm.Title, tm.Description) {
			return false
		}
		for j, lesson := range module.Lessons {
			tl := tm.Lessons[j]
			if !required(tl.Title, tl.Description) {
				return false
			}
			for _, value := range tl.LearningObjectives {
				if value == nil {
					return false
				}
			}
			for k, block := range lesson.Content.Blocks {
				b := tl.ContentBlocks[k]
				switch x := block.Payload.(type) {
				case courses.TextBlockPayload, courses.HeadingBlockPayload:
					if b.Text == nil {
						return false
					}
				case courses.ImageBlockPayload:
					if !required(b.AltText, b.Caption) {
						return false
					}
				case courses.VideoBlockPayload:
					if b.Title == nil || (x.TranscriptAsset == nil && b.Transcript == nil) {
						return false
					}
				case courses.AudioBlockPayload:
					if b.Title == nil || (x.TranscriptAsset == nil && b.Transcript == nil) {
						return false
					}
				case courses.CodeBlockPayload:
					if x.Title != "" && b.Title == nil {
						return false
					}
				case courses.QuoteBlockPayload:
					if !required(b.Text, b.Attribution) {
						return false
					}
				case courses.CalloutBlockPayload:
					if !required(b.Title, b.Text) {
						return false
					}
				case courses.TableBlockPayload:
					if b.Caption == nil {
						return false
					}
				case courses.DownloadBlockPayload:
					if !required(b.Label, b.Description) {
						return false
					}
				}
			}
		}
	}
	for i := range source.AssessmentBindings {
		for j := range source.AssessmentBindings[i].Questions {
			q := t.Assessments[i].Questions[j]
			if q.Prompt == nil {
				return false
			}
			for _, x := range q.Options {
				if x.Text == nil {
					return false
				}
			}
			for _, x := range q.LeftItems {
				if x.Text == nil {
					return false
				}
			}
			for _, x := range q.RightItems {
				if x.Text == nil {
					return false
				}
			}
		}
	}
	return true
}

var _ = fmt.Sprintf
