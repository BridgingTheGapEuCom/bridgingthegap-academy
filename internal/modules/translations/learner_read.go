package translations

import (
	"context"
	"errors"
	"github.com/BridgingTheGapEuCom/bridgingthegap-academy/internal/modules/courses"
)

type TranslationMetadata struct {
	Language       courses.LanguageTag
	SourceLanguage courses.LanguageTag
	SourceVersion  courses.Version
	PublicationID  TranslationPublicationID
	PublishedAt    string
}
type SourceLag struct {
	TranslatedSourceVersion courses.Version
	LatestSourceVersion     *courses.Version
	IsLatest                *bool
}
type LearnerTranslatedCourse struct {
	Course      courses.PublishedCourseVersion
	Translation TranslationMetadata
	Lag         SourceLag
}
type LanguageChoice struct {
	Language      courses.LanguageTag
	Kind          string
	PublicationID *TranslationPublicationID
}
type LearnerReader struct {
	translations *Service
	courses      *courses.PublishedReadService
}

func NewLearnerReader(t *Service, c *courses.PublishedReadService) (*LearnerReader, error) {
	if t == nil || c == nil {
		return nil, ErrInvalidTranslation
	}
	return &LearnerReader{t, c}, nil
}
func (r *LearnerReader) Read(ctx context.Context, courseID courses.CourseID, version courses.Version, language courses.LanguageTag) (LearnerTranslatedCourse, error) {
	source, err := r.translations.PublishedSource(ctx, courseID, version)
	if err != nil {
		return LearnerTranslatedCourse{}, err
	}
	pub, err := r.translations.LatestPublication(ctx, source.ID, language)
	if err != nil {
		return LearnerTranslatedCourse{}, err
	}
	if pub.Source.CourseID != courseID || pub.Source.CourseVersionID != source.ID || pub.Source.Version != version || pub.TargetLanguage != language || pub.Tree.ValidateAgainstSource(source) != nil {
		return LearnerTranslatedCourse{}, ErrSourceIntegrity
	}
	base, err := r.courses.Exact(ctx, courseID, version)
	if err != nil {
		return LearnerTranslatedCourse{}, err
	}
	translated, err := applyPublication(base, pub)
	if err != nil {
		return LearnerTranslatedCourse{}, err
	}
	translated.SourceLanguage = language
	result := LearnerTranslatedCourse{Course: translated, Translation: TranslationMetadata{Language: language, SourceLanguage: pub.Source.Language, SourceVersion: version, PublicationID: pub.ID, PublishedAt: pub.PublishedAt.UTC().Format("2006-01-02T15:04:05Z")}, Lag: SourceLag{TranslatedSourceVersion: version}}
	if latest, latestErr := r.courses.Latest(ctx, courseID); latestErr == nil {
		v := latest.Version
		result.Lag.LatestSourceVersion = &v
		same := v == version
		result.Lag.IsLatest = &same
	}
	return result, nil
}
func (r *LearnerReader) Languages(ctx context.Context, courseID courses.CourseID, version courses.Version) ([]LanguageChoice, error) {
	source, err := r.translations.PublishedSource(ctx, courseID, version)
	if err != nil {
		return nil, err
	}
	langs, err := r.translations.Languages(ctx, source.ID)
	if err != nil {
		return nil, err
	}
	out := []LanguageChoice{{Language: source.CourseVersion.SourceLanguage, Kind: "SOURCE"}}
	for _, l := range langs {
		p, err := r.translations.LatestPublication(ctx, source.ID, l)
		if err != nil {
			return nil, err
		}
		id := p.ID
		out = append(out, LanguageChoice{Language: l, Kind: "TRANSLATION", PublicationID: &id})
	}
	return out, nil
}
func applyPublication(v courses.PublishedCourseVersion, p TranslationPublication) (courses.PublishedCourseVersion, error) {
	t := p.Tree
	if t.Title == nil || t.Description == nil {
		return courses.PublishedCourseVersion{}, ErrSourceIntegrity
	}
	v.Title = *t.Title
	v.Description = *t.Description
	v.LearningObjectives = values(t.LearningObjectives)
	if len(v.Modules) != len(t.Modules) || len(v.Assessments) != len(t.Assessments) {
		return courses.PublishedCourseVersion{}, ErrSourceIntegrity
	}
	for i := range v.Modules {
		m := &v.Modules[i]
		tm := t.Modules[i]
		if m.StableKey != tm.SourceStableKey || tm.Title == nil || tm.Description == nil {
			return courses.PublishedCourseVersion{}, ErrSourceIntegrity
		}
		m.Title = *tm.Title
		m.Description = *tm.Description
		if len(m.Lessons) != len(tm.Lessons) {
			return v, ErrSourceIntegrity
		}
		for j := range m.Lessons {
			l := &m.Lessons[j]
			tl := tm.Lessons[j]
			if l.StableKey != tl.SourceStableKey || tl.Title == nil || tl.Description == nil {
				return v, ErrSourceIntegrity
			}
			l.Title = *tl.Title
			l.Description = *tl.Description
			l.LearningObjectives = values(tl.LearningObjectives)
			if len(l.Content.Blocks) != len(tl.ContentBlocks) {
				return v, ErrSourceIntegrity
			}
			for k := range l.Content.Blocks {
				if l.Content.Blocks[k].Key != tl.ContentBlocks[k].SourceBlockKey {
					return v, ErrSourceIntegrity
				}
				if err := applyBlock(&l.Content.Blocks[k], tl.ContentBlocks[k]); err != nil {
					return v, err
				}
			}
		}
	}
	for i := range v.Assessments {
		a := &v.Assessments[i]
		ta := t.Assessments[i]
		if a.AssessmentKey != ta.AssessmentKey || len(a.Questions) != len(ta.Questions) {
			return v, ErrSourceIntegrity
		}
		for j := range a.Questions {
			q := &a.Questions[j]
			tq := ta.Questions[j]
			if q.StableKey != tq.SourceStableKey || tq.Prompt == nil {
				return v, ErrSourceIntegrity
			}
			q.Prompt = *tq.Prompt
			for k := range q.Options {
				if k >= len(tq.Options) || q.Options[k].StableKey != tq.Options[k].SourceStableKey || tq.Options[k].Text == nil {
					return v, ErrSourceIntegrity
				}
				q.Options[k].Text = *tq.Options[k].Text
			}
			for k := range q.LeftItems {
				if k >= len(tq.LeftItems) || q.LeftItems[k].StableKey != tq.LeftItems[k].SourceStableKey || tq.LeftItems[k].Text == nil {
					return v, ErrSourceIntegrity
				}
				q.LeftItems[k].Text = *tq.LeftItems[k].Text
			}
			for k := range q.RightItems {
				if k >= len(tq.RightItems) || q.RightItems[k].StableKey != tq.RightItems[k].SourceStableKey || tq.RightItems[k].Text == nil {
					return v, ErrSourceIntegrity
				}
				q.RightItems[k].Text = *tq.RightItems[k].Text
			}
		}
	}
	return v, nil
}
func values(in []*string) []string {
	out := make([]string, len(in))
	for i, v := range in {
		if v != nil {
			out[i] = *v
		}
	}
	return out
}
func applyBlock(b *courses.Block, t TranslatedContentBlock) error {
	get := func(p *string) (string, error) {
		if p == nil {
			return "", ErrSourceIntegrity
		}
		return *p, nil
	}
	switch x := b.Payload.(type) {
	case courses.TextBlockPayload:
		v, e := get(t.Text)
		if e != nil {
			return e
		}
		x.Content = plain(v)
		b.Payload = x
	case courses.HeadingBlockPayload:
		v, e := get(t.Text)
		if e != nil {
			return e
		}
		x.Content = []courses.RichTextInline{{Type: "text", Text: v}}
		b.Payload = x
	case courses.QuoteBlockPayload:
		v, e := get(t.Text)
		if e != nil {
			return e
		}
		x.Text = v
		if t.Attribution == nil {
			return ErrSourceIntegrity
		}
		x.Attribution = *t.Attribution
		b.Payload = x
	case courses.CalloutBlockPayload:
		v, e := get(t.Text)
		if e != nil {
			return e
		}
		x.Content = plain(v)
		if t.Title == nil {
			return ErrSourceIntegrity
		}
		x.Title = *t.Title
		b.Payload = x
	case courses.CodeBlockPayload:
		if t.Title != nil {
			x.Title = *t.Title
			b.Payload = x
		}
	case courses.ImageBlockPayload:
		if t.AltText == nil || t.Caption == nil {
			return ErrSourceIntegrity
		}
		x.AltText = *t.AltText
		x.Caption = *t.Caption
		b.Payload = x
	case courses.VideoBlockPayload:
		if t.Title == nil || t.Transcript == nil {
			return ErrSourceIntegrity
		}
		x.Title = *t.Title
		x.Transcript = *t.Transcript
		b.Payload = x
	case courses.AudioBlockPayload:
		if t.Title == nil || t.Transcript == nil {
			return ErrSourceIntegrity
		}
		x.Title = *t.Title
		x.Transcript = *t.Transcript
		b.Payload = x
	case courses.TableBlockPayload:
		if t.Caption == nil {
			return ErrSourceIntegrity
		}
		x.Caption = *t.Caption
		b.Payload = x
	case courses.DownloadBlockPayload:
		if t.Label == nil || t.Description == nil {
			return ErrSourceIntegrity
		}
		x.Label = *t.Label
		x.Description = *t.Description
		b.Payload = x
	}
	return nil
}
func plain(v string) courses.RichText {
	return courses.RichText{Nodes: []courses.RichTextNode{{Type: "paragraph", Content: []courses.RichTextInline{{Type: "text", Text: v}}}}}
}

var _ = errors.New
