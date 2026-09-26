// Package portability builds the versioned Academy course package format.
// It only reads immutable published artifacts. Import is intentionally deferred.
package portability

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"sort"
	"time"

	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/assets"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/translations"
)

const (
	Format          = "bridging-the-gap-course"
	FormatVersion   = 1
	manifestPath    = "manifest.json"
	coursePath      = "course.json"
	assessmentsPath = "assessments.json"
	checksumsPath   = "checksums.json"
)

var (
	ErrExportNotFound  = errors.New("published course version not found for export")
	ErrExportIntegrity = errors.New("published course export integrity failure")
	ErrExportStorage   = errors.New("published asset export storage failure")
)

// SourceRepository is deliberately limited to exact immutable publication
// reads. It prevents the exporter from selecting latest or mutable Authoring.
type SourceRepository interface {
	GetImmutableCourseVersionByCourseAndVersion(context.Context, courses.CourseID, courses.Version) (courses.ImmutableCourseVersion, error)
}

// Manifest is the importer-facing package entry point. Origin IDs are
// provenance hints only: an importer must create its own local identities.
type Manifest struct {
	Format        string             `json:"format"`
	FormatVersion int                `json:"formatVersion"`
	ExportedAt    time.Time          `json:"exportedAt"`
	Course        ManifestCourse     `json:"course"`
	Contents      []ContentEntry     `json:"contents"`
	Assets        []AssetEntry       `json:"assets"`
	Translations  []TranslationEntry `json:"translations"`
	ChecksumsPath string             `json:"checksumsPath"`
}
type ManifestCourse struct {
	OriginCourseID        string        `json:"originCourseId,omitempty"`
	OriginCourseVersionID string        `json:"originCourseVersionId,omitempty"`
	Version               string        `json:"version"`
	Language              string        `json:"language"`
	License               License       `json:"license"`
	Attribution           []Attribution `json:"attribution"`
}
type ContentEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type AssetEntry struct {
	AssetKey         string `json:"assetKey"`
	OriginalFilename string `json:"originalFilename"`
	MediaType        string `json:"mediaType"`
	ByteSize         int64  `json:"byteSize"`
	SHA256           string `json:"sha256"`
	Path             string `json:"path"`
}
type TranslationEntry struct {
	Language      string    `json:"language"`
	Path          string    `json:"path"`
	SourceVersion string    `json:"sourceVersion"`
	PublishedAt   time.Time `json:"publishedAt"`
}
type License struct {
	Kind        string `json:"kind"`
	Identifier  string `json:"identifier,omitempty"`
	DisplayName string `json:"displayName"`
	URL         string `json:"url,omitempty"`
	CustomText  string `json:"customText,omitempty"`
}
type Attribution struct {
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Order       int    `json:"order"`
}

// CoursePayload contains canonical portable course data. Stable keys, not local
// row IDs, connect modules, lessons, content, and prerequisites.
type CoursePayload struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	LearningObjectives []string `json:"learningObjectives"`
	Language           string   `json:"language"`
	Version            string   `json:"version"`
	Changelog          string   `json:"changelog"`
	Modules            []Module `json:"modules"`
}
type Module struct {
	StableKey   string   `json:"stableKey"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Position    int      `json:"position"`
	Lessons     []Lesson `json:"lessons"`
}
type Lesson struct {
	StableKey                string                `json:"stableKey"`
	Title                    string                `json:"title"`
	Description              string                `json:"description"`
	LearningObjectives       []string              `json:"learningObjectives"`
	EstimatedDurationMinutes *int                  `json:"estimatedDurationMinutes,omitempty"`
	Position                 int                   `json:"position"`
	PrerequisiteStableKeys   []string              `json:"prerequisiteStableKeys"`
	Content                  courses.LessonContent `json:"content"`
}
type AssessmentsPayload struct {
	Assessments []Assessment `json:"assessments"`
}
type Assessment struct {
	AssessmentKey string     `json:"assessmentKey"`
	Questions     []Question `json:"questions"`
}
type Question struct {
	StableKey         string                                  `json:"stableKey"`
	Type              courses.PublishedAssessmentQuestionType `json:"type"`
	Prompt            string                                  `json:"prompt"`
	Position          int                                     `json:"position"`
	Options           []Option                                `json:"options,omitempty"`
	CorrectOptionKeys []string                                `json:"correctOptionKeys,omitempty"`
	LeftItems         []Item                                  `json:"leftItems,omitempty"`
	RightItems        []Item                                  `json:"rightItems,omitempty"`
	CorrectPairs      []Pair                                  `json:"correctPairs,omitempty"`
}
type Option struct {
	StableKey string `json:"stableKey"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
}
type Item struct {
	StableKey string `json:"stableKey"`
	Text      string `json:"text"`
	Position  int    `json:"position"`
}
type Pair struct {
	LeftKey  string `json:"leftKey"`
	RightKey string `json:"rightKey"`
}
type TranslationPayload struct {
	Language              string                       `json:"language"`
	SourceCourseID        string                       `json:"sourceCourseId"`
	SourceCourseVersionID string                       `json:"sourceCourseVersionId"`
	SourceVersion         string                       `json:"sourceVersion"`
	PublishedAt           time.Time                    `json:"publishedAt"`
	Tree                  translations.TranslationTree `json:"tree"`
}
type Checksums struct {
	Entries []ContentEntry `json:"entries"`
}

type Exporter struct {
	source       SourceRepository
	translations *translations.Service
	storage      assets.BinaryStorage
	now          func() time.Time
}

func NewExporter(source SourceRepository, translationService *translations.Service, storage assets.BinaryStorage, now func() time.Time) (*Exporter, error) {
	if source == nil || translationService == nil || storage == nil {
		return nil, ErrExportIntegrity
	}
	if now == nil {
		now = time.Now
	}
	return &Exporter{source: source, translations: translationService, storage: storage, now: now}, nil
}

// Write produces a ZIP only after a full first pass verifies every frozen
// asset byte. Callers must treat a returned error as no completed package.
func (e *Exporter) Write(ctx context.Context, courseID courses.CourseID, version courses.Version, out io.Writer) error {
	if e == nil || e.source == nil || e.storage == nil || !version.Valid() || courseID == "" || out == nil {
		return ErrExportIntegrity
	}
	v, err := e.source.GetImmutableCourseVersionByCourseAndVersion(ctx, courseID, version)
	if err != nil {
		return ErrExportNotFound
	}
	if err := validateSource(v, courseID, version); err != nil {
		return err
	}
	for _, binding := range v.AssetBindings {
		if err := e.verifyAsset(ctx, binding); err != nil {
			return err
		}
	}
	payload, assessmentPayload := makePayloads(v)
	files := map[string][]byte{}
	if files[coursePath], err = json.Marshal(payload); err != nil {
		return ErrExportIntegrity
	}
	if files[assessmentsPath], err = json.Marshal(assessmentPayload); err != nil {
		return ErrExportIntegrity
	}
	translationsPayload, translationEntries, err := e.translationsFor(ctx, v)
	if err != nil {
		return err
	}
	for p, body := range translationsPayload {
		files[p] = body
	}
	entries := make([]ContentEntry, 0, len(files)+len(v.AssetBindings))
	for p, body := range files {
		entries = append(entries, ContentEntry{Path: p, SHA256: digestBytes(body)})
	}
	assetEntries := assetEntriesFor(v.AssetBindings)
	for _, a := range assetEntries {
		entries = append(entries, ContentEntry{Path: a.Path, SHA256: a.SHA256})
	}
	sortEntries(entries)
	checksums, err := json.Marshal(Checksums{Entries: entries})
	if err != nil {
		return ErrExportIntegrity
	}
	manifest := Manifest{Format: Format, FormatVersion: FormatVersion, ExportedAt: e.now().UTC(), Course: manifestCourse(v), Contents: entries, Assets: assetEntries, Translations: translationEntries, ChecksumsPath: checksumsPath}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return ErrExportIntegrity
	}
	z := zip.NewWriter(out)
	if err := writeZipBytes(z, manifestPath, manifestBytes); err != nil {
		return err
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := writeZipBytes(z, p, files[p]); err != nil {
			return err
		}
	}
	bindings := make(map[string]courses.PublishedAssetBinding, len(v.AssetBindings))
	for _, binding := range v.AssetBindings {
		bindings[binding.AssetKey] = binding
	}
	for _, entry := range assetEntries {
		if err := e.writeAsset(ctx, z, bindings[entry.AssetKey], entry); err != nil {
			return err
		}
	}
	if err := writeZipBytes(z, checksumsPath, checksums); err != nil {
		return err
	}
	if err := z.Close(); err != nil {
		return ErrExportIntegrity
	}
	return nil
}

func (e *Exporter) translationsFor(ctx context.Context, v courses.ImmutableCourseVersion) (map[string][]byte, []TranslationEntry, error) {
	result := map[string][]byte{}
	sourceID := v.ID
	langs, err := e.translations.Languages(ctx, sourceID)
	if err != nil {
		return nil, nil, ErrExportIntegrity
	}
	entries := make([]TranslationEntry, 0, len(langs))
	for _, lang := range langs {
		p, err := e.translations.LatestPublication(ctx, sourceID, lang)
		if err != nil {
			return nil, nil, ErrExportIntegrity
		}
		if p.Source.CourseID != v.CourseVersion.CourseID || p.Source.CourseVersionID != sourceID || p.Source.Version != v.CourseVersion.Version || p.TargetLanguage != lang || p.Tree.ValidateAgainstSource(v) != nil {
			return nil, nil, ErrExportIntegrity
		}
		file := path.Join("translations", string(lang)+".json")
		body, err := json.Marshal(TranslationPayload{Language: string(lang), SourceCourseID: string(v.CourseVersion.CourseID), SourceCourseVersionID: string(v.ID), SourceVersion: v.CourseVersion.Version.String(), PublishedAt: p.PublishedAt.UTC(), Tree: p.Tree})
		if err != nil {
			return nil, nil, ErrExportIntegrity
		}
		result[file] = body
		entries = append(entries, TranslationEntry{Language: string(lang), Path: file, SourceVersion: v.CourseVersion.Version.String(), PublishedAt: p.PublishedAt.UTC()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Language < entries[j].Language })
	return result, entries, nil
}
func (e *Exporter) verifyAsset(ctx context.Context, b courses.PublishedAssetBinding) error {
	r, err := e.storage.Open(ctx, assets.StorageObjectID(b.StorageObjectID))
	if err != nil {
		return ErrExportStorage
	}
	n, d, err := hashReader(r)
	if closeErr := r.Close(); closeErr != nil {
		return ErrExportStorage
	}
	if err != nil || n != b.ByteSize || d != b.SHA256Digest {
		return ErrExportIntegrity
	}
	return nil
}
func (e *Exporter) writeAsset(ctx context.Context, z *zip.Writer, binding courses.PublishedAssetBinding, entry AssetEntry) error {
	r, err := e.storage.Open(ctx, assets.StorageObjectID(binding.StorageObjectID))
	if err != nil {
		return ErrExportStorage
	}
	h := &zip.FileHeader{Name: entry.Path, Method: zip.Deflate}
	h.Modified = time.Unix(0, 0).UTC()
	w, err := z.CreateHeader(h)
	if err != nil {
		return ErrExportIntegrity
	}
	n, digest, err := hashCopy(w, r)
	if closeErr := r.Close(); closeErr != nil {
		return ErrExportStorage
	}
	if err != nil || n != entry.ByteSize || digest != entry.SHA256 {
		return ErrExportIntegrity
	}
	return nil
}
func writeZipBytes(z *zip.Writer, p string, b []byte) error {
	h := &zip.FileHeader{Name: p, Method: zip.Deflate}
	h.Modified = time.Unix(0, 0).UTC()
	w, err := z.CreateHeader(h)
	if err != nil {
		return ErrExportIntegrity
	}
	if _, err = w.Write(b); err != nil {
		return ErrExportIntegrity
	}
	return nil
}
func digestBytes(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func hashReader(r io.Reader) (int64, string, error) {
	h := sha256.New()
	n, e := io.Copy(h, r)
	return n, hex.EncodeToString(h.Sum(nil)), e
}
func hashCopy(dst io.Writer, src io.Reader) (int64, string, error) {
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(dst, h), src)
	return n, hex.EncodeToString(h.Sum(nil)), e
}
func sortEntries(x []ContentEntry) {
	sort.Slice(x, func(i, j int) bool { return x[i].Path < x[j].Path })
}
func manifestCourse(v courses.ImmutableCourseVersion) ManifestCourse {
	m := v.CourseVersion
	a := make([]Attribution, 0, len(m.Attribution))
	for _, c := range m.Attribution {
		a = append(a, Attribution{DisplayName: c.DisplayName, Role: string(c.Role), Order: c.Order})
	}
	return ManifestCourse{OriginCourseID: string(m.CourseID), OriginCourseVersionID: string(v.ID), Version: m.Version.String(), Language: string(m.SourceLanguage), License: License{Kind: string(m.License.Kind), Identifier: m.License.Identifier, DisplayName: m.License.DisplayName, URL: m.License.URL, CustomText: m.License.CustomText}, Attribution: a}
}
func assetEntriesFor(bindings []courses.PublishedAssetBinding) []AssetEntry {
	out := make([]AssetEntry, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, AssetEntry{AssetKey: b.AssetKey, OriginalFilename: b.OriginalFilename, MediaType: b.MediaType, ByteSize: b.ByteSize, SHA256: b.SHA256Digest, Path: path.Join("assets", b.AssetKey, "content")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AssetKey < out[j].AssetKey })
	return out
}
func makePayloads(v courses.ImmutableCourseVersion) (CoursePayload, AssessmentsPayload) {
	m := v.CourseVersion
	p := CoursePayload{Title: m.Title, Description: m.Description, LearningObjectives: append([]string(nil), m.LearningObjectives...), Language: string(m.SourceLanguage), Version: m.Version.String(), Changelog: m.Changelog, Modules: make([]Module, 0, len(v.Modules))}
	for _, mod := range v.Modules {
		x := Module{StableKey: mod.StableKey, Title: mod.Title, Description: mod.Description, Position: mod.Position, Lessons: make([]Lesson, 0, len(mod.Lessons))}
		for _, l := range mod.Lessons {
			x.Lessons = append(x.Lessons, Lesson{StableKey: l.StableKey, Title: l.Title, Description: l.Description, LearningObjectives: append([]string(nil), l.LearningObjectives...), EstimatedDurationMinutes: l.EstimatedDurationMinutes, Position: l.Position, PrerequisiteStableKeys: append([]string(nil), l.PrerequisiteStableKeys...), Content: l.Content})
		}
		p.Modules = append(p.Modules, x)
	}
	a := AssessmentsPayload{Assessments: make([]Assessment, 0, len(v.AssessmentBindings))}
	for _, b := range v.AssessmentBindings {
		x := Assessment{AssessmentKey: b.AssessmentKey, Questions: make([]Question, 0, len(b.Questions))}
		for _, q := range b.Questions {
			z := Question{StableKey: q.StableKey, Type: q.Type, Prompt: q.Prompt, Position: q.Position, CorrectOptionKeys: append([]string(nil), q.CorrectOptionKeys...), CorrectPairs: make([]Pair, 0, len(q.CorrectPairs))}
			for _, o := range q.Options {
				z.Options = append(z.Options, Option(o))
			}
			for _, i := range q.LeftItems {
				z.LeftItems = append(z.LeftItems, Item(i))
			}
			for _, i := range q.RightItems {
				z.RightItems = append(z.RightItems, Item(i))
			}
			for _, pair := range q.CorrectPairs {
				z.CorrectPairs = append(z.CorrectPairs, Pair(pair))
			}
			x.Questions = append(x.Questions, z)
		}
		a.Assessments = append(a.Assessments, x)
	}
	return p, a
}
func validateSource(v courses.ImmutableCourseVersion, courseID courses.CourseID, version courses.Version) error {
	if v.ID == "" || v.CourseVersion.Status != courses.CourseVersionPublished || v.CourseVersion.CourseID != courseID || v.CourseVersion.Version != version || v.CourseVersion.Validate() != nil {
		return ErrExportIntegrity
	}
	lessons, assetKeys, assessmentKeys := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, b := range v.AssetBindings {
		if b.Validate() != nil || assetKeys[b.AssetKey] {
			return ErrExportIntegrity
		}
		assetKeys[b.AssetKey] = true
	}
	for _, b := range v.AssessmentBindings {
		if b.Validate() != nil || assessmentKeys[b.AssessmentKey] {
			return ErrExportIntegrity
		}
		assessmentKeys[b.AssessmentKey] = true
	}
	for _, m := range v.Modules {
		for _, l := range m.Lessons {
			if l.Content.Validate() != nil || lessons[l.StableKey] {
				return ErrExportIntegrity
			}
			lessons[l.StableKey] = true
		}
	}
	for _, m := range v.Modules {
		for _, l := range m.Lessons {
			for _, key := range l.PrerequisiteStableKeys {
				if !lessons[key] {
					return ErrExportIntegrity
				}
			}
			for _, block := range l.Content.Blocks {
				switch x := block.Payload.(type) {
				case courses.ImageBlockPayload:
					if !assetKeys[x.Asset.AssetKey] {
						return ErrExportIntegrity
					}
				case courses.VideoBlockPayload:
					if !assetKeys[x.Asset.AssetKey] || !assetKeys[x.CaptionsAsset.AssetKey] || (x.TranscriptAsset != nil && !assetKeys[x.TranscriptAsset.AssetKey]) {
						return ErrExportIntegrity
					}
				case courses.AudioBlockPayload:
					if !assetKeys[x.Asset.AssetKey] || (x.TranscriptAsset != nil && !assetKeys[x.TranscriptAsset.AssetKey]) {
						return ErrExportIntegrity
					}
				case courses.DownloadBlockPayload:
					if !assetKeys[x.Asset.AssetKey] {
						return ErrExportIntegrity
					}
				case courses.KnowledgeCheckBlockPayload:
					if !assessmentKeys[x.AssessmentKey] {
						return ErrExportIntegrity
					}
				}
			}
		}
	}
	return nil
}
