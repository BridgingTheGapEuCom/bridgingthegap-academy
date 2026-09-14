package courses

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	LessonContentSchemaVersion = 1
	MaxLessonContentBytes      = 1 << 20
	MaxLessonBlocks            = 200
	MaxBlockTextCharacters     = 50000
	MaxCodeCharacters          = 100000
	MaxURLLength               = 2048
	MaxTableColumns            = 20
	MaxTableRows               = 200
)

type BlockType string

const (
	BlockText           BlockType = "TEXT"
	BlockHeading        BlockType = "HEADING"
	BlockImage          BlockType = "IMAGE"
	BlockVideo          BlockType = "VIDEO"
	BlockAudio          BlockType = "AUDIO"
	BlockCode           BlockType = "CODE"
	BlockQuote          BlockType = "QUOTE"
	BlockCallout        BlockType = "CALLOUT"
	BlockTable          BlockType = "TABLE"
	BlockDownload       BlockType = "DOWNLOAD"
	BlockKnowledgeCheck BlockType = "KNOWLEDGE_CHECK"
	BlockDivider        BlockType = "DIVIDER"
)

type LessonContent struct {
	SchemaVersion int     `json:"schemaVersion"`
	Blocks        []Block `json:"blocks"`
}

func (c LessonContent) Validate() error {
	if c.SchemaVersion != 1 {
		return errors.New("unsupported lesson content schema version")
	}
	if len(c.Blocks) > MaxLessonBlocks {
		return errors.New("too many lesson blocks")
	}
	seen := map[string]bool{}
	for _, b := range c.Blocks {
		k, e := NormalizeStructureKey(b.Key)
		if e != nil || k != b.Key {
			return errors.New("invalid lesson block key")
		}
		if seen[b.Key] {
			return errors.New("duplicate lesson block key")
		}
		seen[b.Key] = true
		if e := b.Validate(); e != nil {
			return e
		}
	}
	return nil
}
func MarshalLessonContent(c LessonContent) ([]byte, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	b, e := json.Marshal(c)
	if e != nil || len(b) > MaxLessonContentBytes {
		return nil, errors.New("invalid lesson content")
	}
	return b, nil
}
func ParseLessonContent(b []byte) (LessonContent, error) {
	if len(b) == 0 || len(b) > MaxLessonContentBytes {
		return LessonContent{}, errors.New("invalid lesson content size")
	}
	var c LessonContent
	if e := strict(b, &c); e != nil {
		return LessonContent{}, errors.New("invalid lesson content document")
	}
	return c, c.Validate()
}

type BlockPayload interface {
	blockPayload()
	Validate() error
}
type Block struct {
	Key     string
	Type    BlockType
	Payload BlockPayload
}

func (b Block) Validate() error {
	switch b.Type {
	case BlockText:
		return expect[TextBlockPayload](b.Payload)
	case BlockHeading:
		return expect[HeadingBlockPayload](b.Payload)
	case BlockImage:
		return expect[ImageBlockPayload](b.Payload)
	case BlockVideo:
		return expect[VideoBlockPayload](b.Payload)
	case BlockAudio:
		return expect[AudioBlockPayload](b.Payload)
	case BlockCode:
		return expect[CodeBlockPayload](b.Payload)
	case BlockQuote:
		return expect[QuoteBlockPayload](b.Payload)
	case BlockCallout:
		return expect[CalloutBlockPayload](b.Payload)
	case BlockTable:
		return expect[TableBlockPayload](b.Payload)
	case BlockDownload:
		return expect[DownloadBlockPayload](b.Payload)
	case BlockKnowledgeCheck:
		return expect[KnowledgeCheckBlockPayload](b.Payload)
	case BlockDivider:
		return expect[DividerBlockPayload](b.Payload)
	}
	return errors.New("unsupported lesson block type")
}
func expect[T BlockPayload](p BlockPayload) error {
	v, ok := p.(T)
	if !ok {
		return errors.New("block payload does not match type")
	}
	return v.Validate()
}
func (b Block) MarshalJSON() ([]byte, error) {
	if e := b.Validate(); e != nil {
		return nil, e
	}
	return json.Marshal(struct {
		Key     string       `json:"key"`
		Type    BlockType    `json:"type"`
		Payload BlockPayload `json:"payload"`
	}{b.Key, b.Type, b.Payload})
}
func (b *Block) UnmarshalJSON(data []byte) error {
	var r struct {
		Key     string          `json:"key"`
		Type    BlockType       `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if e := strict(data, &r); e != nil || len(r.Payload) == 0 {
		return errors.New("invalid lesson block")
	}
	p, e := blockPayload(r.Type, r.Payload)
	if e != nil {
		return e
	}
	b.Key, b.Type, b.Payload = r.Key, r.Type, p
	return nil
}
func blockPayload(t BlockType, b []byte) (BlockPayload, error) {
	switch t {
	case BlockText:
		return decode[TextBlockPayload](b)
	case BlockHeading:
		return decode[HeadingBlockPayload](b)
	case BlockImage:
		return decode[ImageBlockPayload](b)
	case BlockVideo:
		return decode[VideoBlockPayload](b)
	case BlockAudio:
		return decode[AudioBlockPayload](b)
	case BlockCode:
		return decode[CodeBlockPayload](b)
	case BlockQuote:
		return decode[QuoteBlockPayload](b)
	case BlockCallout:
		return decode[CalloutBlockPayload](b)
	case BlockTable:
		return decode[TableBlockPayload](b)
	case BlockDownload:
		return decode[DownloadBlockPayload](b)
	case BlockKnowledgeCheck:
		return decode[KnowledgeCheckBlockPayload](b)
	case BlockDivider:
		return decode[DividerBlockPayload](b)
	}
	return nil, errors.New("unsupported lesson block type")
}
func decode[T BlockPayload](b []byte) (T, error) { var v T; return v, strict(b, &v) }
func strict(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

type RichText struct {
	Nodes []RichTextNode `json:"nodes"`
}
type RichTextNode struct {
	Type    string             `json:"type"`
	Content []RichTextInline   `json:"content,omitempty"`
	Items   [][]RichTextInline `json:"items,omitempty"`
}
type RichTextInline struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	Marks []RichTextMark `json:"marks,omitempty"`
}
type RichTextMark struct {
	Type string `json:"type"`
	Href string `json:"href,omitempty"`
}

func (r RichText) Validate() error {
	if len(r.Nodes) == 0 || len(r.Nodes) > 100 {
		return errors.New("invalid rich text")
	}
	n := 0
	for _, x := range r.Nodes {
		switch x.Type {
		case "paragraph":
			c, e := inlines(x.Content)
			if e != nil || len(x.Items) > 0 {
				return errors.New("invalid paragraph")
			}
			n += c
		case "bullet_list", "ordered_list":
			if len(x.Content) > 0 || len(x.Items) == 0 || len(x.Items) > 100 {
				return errors.New("invalid list")
			}
			for _, i := range x.Items {
				c, e := inlines(i)
				if e != nil {
					return e
				}
				n += c
			}
		default:
			return errors.New("unsupported rich text node")
		}
		if n > MaxBlockTextCharacters {
			return errors.New("rich text too large")
		}
	}
	if n == 0 {
		return errors.New("rich text requires text")
	}
	return nil
}
func inlines(x []RichTextInline) (int, error) {
	if len(x) == 0 || len(x) > 200 {
		return 0, errors.New("invalid rich text inline")
	}
	n := 0
	for _, i := range x {
		if i.Type == "hard_break" {
			if i.Text != "" || len(i.Marks) > 0 {
				return 0, errors.New("invalid hard break")
			}
			continue
		}
		if i.Type != "text" || i.Text == "" || len(i.Marks) > 4 {
			return 0, errors.New("invalid rich text inline")
		}
		n += utf8.RuneCountInString(i.Text)
		for _, m := range i.Marks {
			if m.Type == "link" {
				if e := safeURL(m.Href); e != nil {
					return 0, e
				}
			} else if (m.Type != "emphasis" && m.Type != "strong" && m.Type != "inline_code") || m.Href != "" {
				return 0, errors.New("invalid rich text mark")
			}
		}
	}
	return n, nil
}

type TextBlockPayload struct {
	Content RichText `json:"content"`
}

func (TextBlockPayload) blockPayload()     {}
func (p TextBlockPayload) Validate() error { return p.Content.Validate() }

type HeadingBlockPayload struct {
	Level   int              `json:"level"`
	Content []RichTextInline `json:"content"`
}

func (HeadingBlockPayload) blockPayload() {}
func (p HeadingBlockPayload) Validate() error {
	if p.Level < 2 || p.Level > 4 {
		return errors.New("invalid heading level")
	}
	_, e := inlines(p.Content)
	return e
}

type AssetReference struct {
	AssetKey string `json:"assetKey"`
}

func (a AssetReference) Validate() error {
	k, e := NormalizeStructureKey(a.AssetKey)
	if e != nil || k != a.AssetKey {
		return errors.New("invalid asset reference")
	}
	return nil
}

type ImageBlockPayload struct {
	Asset      AssetReference `json:"asset"`
	AltText    string         `json:"altText,omitempty"`
	Decorative bool           `json:"decorative"`
	Caption    string         `json:"caption,omitempty"`
}

func (ImageBlockPayload) blockPayload() {}
func (p ImageBlockPayload) Validate() error {
	if e := p.Asset.Validate(); e != nil {
		return e
	}
	if len(p.AltText) > 1000 || len(p.Caption) > 4000 || (p.Decorative && p.AltText != "") || (!p.Decorative && strings.TrimSpace(p.AltText) == "") {
		return errors.New("invalid image accessibility metadata")
	}
	return nil
}

type VideoBlockPayload struct {
	Asset           AssetReference  `json:"asset"`
	Title           string          `json:"title"`
	Transcript      string          `json:"transcript,omitempty"`
	TranscriptAsset *AssetReference `json:"transcriptAsset,omitempty"`
	CaptionsAsset   AssetReference  `json:"captionsAsset"`
}

func (VideoBlockPayload) blockPayload() {}
func (p VideoBlockPayload) Validate() error {
	if e := p.Asset.Validate(); e != nil {
		return e
	}
	if e := p.CaptionsAsset.Validate(); e != nil {
		return e
	}
	if !title(p.Title) || len(p.Transcript) > MaxBlockTextCharacters || (strings.TrimSpace(p.Transcript) == "" && p.TranscriptAsset == nil) {
		return errors.New("invalid video accessibility metadata")
	}
	if p.TranscriptAsset != nil {
		return p.TranscriptAsset.Validate()
	}
	return nil
}

type AudioBlockPayload struct {
	Asset           AssetReference  `json:"asset"`
	Title           string          `json:"title"`
	Transcript      string          `json:"transcript,omitempty"`
	TranscriptAsset *AssetReference `json:"transcriptAsset,omitempty"`
}

func (AudioBlockPayload) blockPayload() {}
func (p AudioBlockPayload) Validate() error {
	if e := p.Asset.Validate(); e != nil {
		return e
	}
	if !title(p.Title) || len(p.Transcript) > MaxBlockTextCharacters || (strings.TrimSpace(p.Transcript) == "" && p.TranscriptAsset == nil) {
		return errors.New("invalid audio accessibility metadata")
	}
	if p.TranscriptAsset != nil {
		return p.TranscriptAsset.Validate()
	}
	return nil
}

type CodeBlockPayload struct {
	Code     string `json:"code"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
}

func (CodeBlockPayload) blockPayload() {}
func (p CodeBlockPayload) Validate() error {
	if p.Code == "" || len(p.Code) > MaxCodeCharacters || len(p.Language) > 64 || len(p.Title) > 240 {
		return errors.New("invalid code block")
	}
	return nil
}

type QuoteBlockPayload struct {
	Text        string `json:"text"`
	Attribution string `json:"attribution,omitempty"`
	SourceURL   string `json:"sourceUrl,omitempty"`
}

func (QuoteBlockPayload) blockPayload() {}
func (p QuoteBlockPayload) Validate() error {
	if strings.TrimSpace(p.Text) == "" || len(p.Text) > MaxBlockTextCharacters || len(p.Attribution) > 240 {
		return errors.New("invalid quote")
	}
	if p.SourceURL != "" {
		return safeURL(p.SourceURL)
	}
	return nil
}

type CalloutBlockPayload struct {
	Kind    string   `json:"kind"`
	Title   string   `json:"title,omitempty"`
	Content RichText `json:"content"`
}

func (CalloutBlockPayload) blockPayload() {}
func (p CalloutBlockPayload) Validate() error {
	if (p.Kind != "INFO" && p.Kind != "NOTE" && p.Kind != "WARNING" && p.Kind != "TIP") || len(p.Title) > 240 {
		return errors.New("invalid callout")
	}
	return p.Content.Validate()
}

type TableBlockPayload struct {
	Caption string     `json:"caption,omitempty"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

func (TableBlockPayload) blockPayload() {}
func (p TableBlockPayload) Validate() error {
	if len(p.Caption) > 1000 || len(p.Headers) == 0 || len(p.Headers) > MaxTableColumns || len(p.Rows) == 0 || len(p.Rows) > MaxTableRows {
		return errors.New("invalid table")
	}
	for _, h := range p.Headers {
		if strings.TrimSpace(h) == "" || len(h) > 2000 {
			return errors.New("invalid table header")
		}
	}
	for _, r := range p.Rows {
		if len(r) != len(p.Headers) {
			return errors.New("table must be rectangular")
		}
		for _, c := range r {
			if len(c) > 2000 {
				return errors.New("table cell too large")
			}
		}
	}
	return nil
}

type DownloadBlockPayload struct {
	Asset       AssetReference `json:"asset"`
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
}

func (DownloadBlockPayload) blockPayload() {}
func (p DownloadBlockPayload) Validate() error {
	if e := p.Asset.Validate(); e != nil {
		return e
	}
	if !title(p.Label) || len(p.Description) > 4000 {
		return errors.New("invalid download")
	}
	return nil
}

type KnowledgeCheckBlockPayload struct {
	AssessmentKey string `json:"assessmentKey"`
}

func (KnowledgeCheckBlockPayload) blockPayload() {}
func (p KnowledgeCheckBlockPayload) Validate() error {
	k, e := NormalizeStructureKey(p.AssessmentKey)
	if e != nil || k != p.AssessmentKey {
		return errors.New("invalid knowledge check reference")
	}
	return nil
}

type DividerBlockPayload struct{}

func (DividerBlockPayload) blockPayload()   {}
func (DividerBlockPayload) Validate() error { return nil }
func title(s string) bool                   { return strings.TrimSpace(s) != "" && len(s) <= 240 }
func safeURL(s string) error {
	if len(s) == 0 || len(s) > MaxURLLength || strings.IndexFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return errors.New("invalid content URL")
	}
	if strings.HasPrefix(s, "/") {
		if strings.HasPrefix(s, "//") {
			return errors.New("invalid content URL")
		}
		return nil
	}
	u, e := url.ParseRequestURI(s)
	if e != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("invalid content URL")
	}
	return nil
}
