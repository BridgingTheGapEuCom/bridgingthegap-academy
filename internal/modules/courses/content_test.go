package courses

import (
	"encoding/json"
	"strings"
	"testing"
)

func validRichText() RichText {
	return RichText{Nodes: []RichTextNode{{Type: "paragraph", Content: []RichTextInline{{Type: "text", Text: "Published text."}}}}}
}

func TestLessonContentValidationAndRoundTrip(t *testing.T) {
	content := LessonContent{SchemaVersion: LessonContentSchemaVersion, Blocks: []Block{
		{Key: "intro", Type: BlockText, Payload: TextBlockPayload{Content: validRichText()}},
		{Key: "divider", Type: BlockDivider, Payload: DividerBlockPayload{}},
	}}
	data, err := MarshalLessonContent(content)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ParseLessonContent(data)
	if err != nil || len(loaded.Blocks) != 2 || loaded.Blocks[0].Key != "intro" || loaded.Blocks[1].Key != "divider" {
		t.Fatalf("content round trip failed: %#v, %v", loaded, err)
	}
	if err := (LessonContent{SchemaVersion: 2}).Validate(); err == nil {
		t.Fatal("unsupported schema version accepted")
	}
	duplicate := content
	duplicate.Blocks = append(duplicate.Blocks, content.Blocks[0])
	if err := duplicate.Validate(); err == nil {
		t.Fatal("duplicate block key accepted")
	}
	if err := (LessonContent{SchemaVersion: 1, Blocks: []Block{{Key: "bad key", Type: BlockDivider, Payload: DividerBlockPayload{}}}}).Validate(); err == nil {
		t.Fatal("invalid block key accepted")
	}
	if err := (LessonContent{SchemaVersion: 1, Blocks: []Block{{Key: "unknown", Type: "WIDGET", Payload: DividerBlockPayload{}}}}).Validate(); err == nil {
		t.Fatal("unknown block type accepted")
	}
	tooMany := LessonContent{SchemaVersion: 1, Blocks: make([]Block, MaxLessonBlocks+1)}
	if err := tooMany.Validate(); err == nil {
		t.Fatal("block limit accepted")
	}
	if _, err := ParseLessonContent([]byte(`{"schemaVersion":1,"blocks":[],"unexpected":true}`)); err == nil {
		t.Fatal("unknown JSON field accepted")
	}
	for _, input := range []string{`{"schemaVersion":1}`, `{"schemaVersion":1,"blocks":null}`, `{"schemaVersion":1,"blocks":[{"key":"divider","type":"DIVIDER","payload":null}]}`} {
		if _, err := ParseLessonContent([]byte(input)); err == nil {
			t.Fatalf("missing or null document field accepted: %s", input)
		}
	}
}

func TestBuiltInBlockPayloadValidation(t *testing.T) {
	asset := AssetReference{AssetKey: "media-asset"}
	opaqueAsset := AssetReference{AssetKey: "11111111-1111-4111-8111-111111111111"}
	if err := opaqueAsset.Validate(); err != nil || strings.Contains(opaqueAsset.AssetKey, "/") || strings.Contains(opaqueAsset.AssetKey, "://") {
		t.Fatalf("canonical asset reference cannot carry an opaque path-free Asset ID: %v", err)
	}
	blocks := []Block{
		{Key: "text", Type: BlockText, Payload: TextBlockPayload{Content: validRichText()}},
		{Key: "heading", Type: BlockHeading, Payload: HeadingBlockPayload{Level: 2, Content: []RichTextInline{{Type: "text", Text: "Heading"}}}},
		{Key: "image", Type: BlockImage, Payload: ImageBlockPayload{Asset: asset, AltText: "Event flow diagram"}},
		{Key: "video", Type: BlockVideo, Payload: VideoBlockPayload{Asset: asset, Title: "Video", Transcript: "Transcript", CaptionsAsset: AssetReference{AssetKey: "video-captions"}}},
		{Key: "audio", Type: BlockAudio, Payload: AudioBlockPayload{Asset: asset, Title: "Audio", Transcript: "Transcript"}},
		{Key: "code", Type: BlockCode, Payload: CodeBlockPayload{Code: "fmt.Println()", Language: "go"}},
		{Key: "quote", Type: BlockQuote, Payload: QuoteBlockPayload{Text: "A quotation.", SourceURL: "https://example.org/source"}},
		{Key: "callout", Type: BlockCallout, Payload: CalloutBlockPayload{Kind: "INFO", Content: validRichText()}},
		{Key: "table", Type: BlockTable, Payload: TableBlockPayload{Headers: []string{"Name"}, Rows: [][]string{{"Value"}}}},
		{Key: "download", Type: BlockDownload, Payload: DownloadBlockPayload{Asset: asset, Label: "Download"}},
		{Key: "knowledge-check", Type: BlockKnowledgeCheck, Payload: KnowledgeCheckBlockPayload{AssessmentKey: "sync-basics-check"}},
		{Key: "divider", Type: BlockDivider, Payload: DividerBlockPayload{}},
	}
	for _, block := range blocks {
		if err := block.Validate(); err != nil {
			t.Fatalf("%s rejected: %v", block.Type, err)
		}
	}
	if err := (ImageBlockPayload{Asset: asset}).Validate(); err == nil {
		t.Fatal("image without alt accepted")
	}
	if err := (ImageBlockPayload{Asset: asset, Decorative: true, AltText: "contradiction"}).Validate(); err == nil {
		t.Fatal("decorative image with alt accepted")
	}
	if err := (HeadingBlockPayload{Level: 1, Content: []RichTextInline{{Type: "text", Text: "H1"}}}).Validate(); err == nil {
		t.Fatal("heading H1 accepted")
	}
	if err := (TableBlockPayload{Headers: []string{"A"}, Rows: [][]string{{"x", "y"}}}).Validate(); err == nil {
		t.Fatal("non-rectangular table accepted")
	}
	if err := (AudioBlockPayload{Asset: asset, Title: "Audio"}).Validate(); err == nil {
		t.Fatal("audio without transcript accepted")
	}
	if err := (VideoBlockPayload{Asset: asset, Title: "Video", Transcript: "text"}).Validate(); err == nil {
		t.Fatal("video without captions reference accepted")
	}
	if err := (QuoteBlockPayload{Text: "x", SourceURL: "javascript:alert(1)"}).Validate(); err == nil {
		t.Fatal("unsafe URL accepted")
	}
	if err := (QuoteBlockPayload{Text: "x", SourceURL: "/\\evil.example"}).Validate(); err == nil {
		t.Fatal("backslash authority URL accepted")
	}
	if err := (CodeBlockPayload{Code: strings.Repeat("x", MaxCodeCharacters+1)}).Validate(); err == nil {
		t.Fatal("oversized code accepted")
	}
	if err := (TableBlockPayload{Headers: make([]string, MaxTableColumns+1), Rows: [][]string{{}}}).Validate(); err == nil {
		t.Fatal("oversized table accepted")
	}
	deep := []byte(`{"schemaVersion":1,"blocks":[{"key":"text","type":"TEXT","payload":{"content":{"nodes":[{"type":"paragraph","content":[{"type":"text","text":"x","content":[]}]}]}}}]}`)
	if _, err := ParseLessonContent(deep); err == nil {
		t.Fatal("unsupported rich-text nesting accepted")
	}
}

func FuzzParseLessonContent(f *testing.F) {
	valid, _ := json.Marshal(testLessonContent())
	f.Add(string(valid))
	f.Add(`{"schemaVersion":1,"blocks":[]}`)
	f.Add(`{"schemaVersion":1,"blocks":[{"key":"table","type":"TABLE","payload":{"headers":["A"],"rows":[["B"]]}}]}`)
	f.Add(`{"schemaVersion":1,"blocks":[{"key":"quote","type":"QUOTE","payload":{"text":"x","sourceUrl":"javascript:alert(1)"}}]}`)
	f.Add(`{"schemaVersion":1,"blocks":[{"key":"divider","type":"DIVIDER","payload":null}]}`)
	f.Add(`{`)
	f.Fuzz(func(t *testing.T, input string) { _, _ = ParseLessonContent([]byte(input)) })
}
