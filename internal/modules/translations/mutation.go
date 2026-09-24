package translations

import "fmt"

// TextChange is the closed, source-keyed mutation vocabulary used by transports.
// It deliberately cannot represent structure, assets, assessment correctness, or
// arbitrary JSON paths.
type TextChange struct {
	Target        string
	Field         string
	ModuleKey     string
	LessonKey     string
	BlockKey      string
	AssessmentKey string
	QuestionKey   string
	ItemKey       string
	Objective     int
	Translated    *string
}

func ApplyTextChanges(tree TranslationTree, changes []TextChange) (TranslationTree, error) {
	if len(changes) == 0 {
		return TranslationTree{}, ErrInvalidTranslation
	}
	copy, err := cloneTree(tree)
	if err != nil {
		return TranslationTree{}, ErrInvalidTranslation
	}
	for _, c := range changes {
		if err := applyTextChange(&copy, c); err != nil {
			return TranslationTree{}, err
		}
	}
	if err := copy.ValidateForPersistence(); err != nil {
		return TranslationTree{}, ErrInvalidTranslation
	}
	return copy, nil
}
func applyTextChange(t *TranslationTree, c TextChange) error {
	set := func(p **string) { *p = c.Translated }
	switch c.Target {
	case "COURSE":
		switch c.Field {
		case "title":
			set(&t.Title)
		case "description":
			set(&t.Description)
		case "objective":
			if c.Objective < 0 || c.Objective >= len(t.LearningObjectives) {
				return ErrInvalidTranslation
			}
			t.LearningObjectives[c.Objective] = c.Translated
		default:
			return ErrInvalidTranslation
		}
		return nil
	case "MODULE":
		m := findModule(t, c.ModuleKey)
		if m == nil {
			return ErrInvalidTranslation
		}
		switch c.Field {
		case "title":
			set(&m.Title)
		case "description":
			set(&m.Description)
		default:
			return ErrInvalidTranslation
		}
		return nil
	case "LESSON":
		l := findLesson(t, c.ModuleKey, c.LessonKey)
		if l == nil {
			return ErrInvalidTranslation
		}
		switch c.Field {
		case "title":
			set(&l.Title)
		case "description":
			set(&l.Description)
		case "objective":
			if c.Objective < 0 || c.Objective >= len(l.LearningObjectives) {
				return ErrInvalidTranslation
			}
			l.LearningObjectives[c.Objective] = c.Translated
		default:
			return ErrInvalidTranslation
		}
		return nil
	case "BLOCK":
		l := findLesson(t, c.ModuleKey, c.LessonKey)
		if l == nil {
			return ErrInvalidTranslation
		}
		for i := range l.ContentBlocks {
			b := &l.ContentBlocks[i]
			if b.SourceBlockKey != c.BlockKey {
				continue
			}
			switch c.Field {
			case "text":
				set(&b.Text)
			case "title":
				set(&b.Title)
			case "altText":
				set(&b.AltText)
			case "caption":
				set(&b.Caption)
			case "attribution":
				set(&b.Attribution)
			case "transcript":
				set(&b.Transcript)
			case "label":
				set(&b.Label)
			case "description":
				set(&b.Description)
			default:
				return ErrInvalidTranslation
			}
			return nil
		}
		return ErrInvalidTranslation
	case "ASSESSMENT_QUESTION", "ASSESSMENT_OPTION", "ASSESSMENT_LEFT_ITEM", "ASSESSMENT_RIGHT_ITEM":
		for ai := range t.Assessments {
			a := &t.Assessments[ai]
			if a.AssessmentKey != c.AssessmentKey {
				continue
			}
			for qi := range a.Questions {
				q := &a.Questions[qi]
				if q.SourceStableKey != c.QuestionKey {
					continue
				}
				if c.Target == "ASSESSMENT_QUESTION" && c.Field == "prompt" {
					q.Prompt = c.Translated
					return nil
				}
				if c.Field != "text" {
					return ErrInvalidTranslation
				}
				var items *[]TranslatedAssessmentItem
				if c.Target == "ASSESSMENT_OPTION" {
					for oi := range q.Options {
						if q.Options[oi].SourceStableKey == c.ItemKey {
							q.Options[oi].Text = c.Translated
							return nil
						}
					}
					return ErrInvalidTranslation
				}
				if c.Target == "ASSESSMENT_LEFT_ITEM" {
					items = &q.LeftItems
				} else {
					items = &q.RightItems
				}
				for ii := range *items {
					if (*items)[ii].SourceStableKey == c.ItemKey {
						(*items)[ii].Text = c.Translated
						return nil
					}
				}
				return ErrInvalidTranslation
			}
			return ErrInvalidTranslation
		}
		return ErrInvalidTranslation
	default:
		return fmt.Errorf("%w: unknown translation target", ErrInvalidTranslation)
	}
}
func findModule(t *TranslationTree, key string) *TranslatedModule {
	for i := range t.Modules {
		if t.Modules[i].SourceStableKey == key {
			return &t.Modules[i]
		}
	}
	return nil
}
func findLesson(t *TranslationTree, module, lesson string) *TranslatedLesson {
	m := findModule(t, module)
	if m == nil {
		return nil
	}
	for i := range m.Lessons {
		if m.Lessons[i].SourceStableKey == lesson {
			return &m.Lessons[i]
		}
	}
	return nil
}
