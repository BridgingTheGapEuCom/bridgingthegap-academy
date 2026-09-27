package plugins

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func intPointer(value int) *int           { return &value }
func floatPointer(value float64) *float64 { return &value }

func TestConfigurationSchemaValidation(t *testing.T) {
	valid := &ConfigurationSchema{Fields: []ConfigurationField{
		{Key: "title", Type: ConfigurationText, Label: "Title", Required: true, Default: json.RawMessage(`"Overview"`), MinLength: intPointer(2), MaxLength: intPointer(80)},
		{Key: "notes", Type: ConfigurationTextarea, Label: "Notes"},
		{Key: "count", Type: ConfigurationInteger, Label: "Number of items", Min: floatPointer(1), Max: floatPointer(20)},
		{Key: "ratio", Type: ConfigurationNumber, Label: "Ratio"},
		{Key: "completed", Type: ConfigurationBoolean, Label: "Show completed", Default: json.RawMessage(`true`)},
		{Key: "scope", Type: ConfigurationSingleSelect, Label: "Scope", Options: []ConfigurationOption{{Value: "all", Label: "All courses"}, {Value: "active", Label: "Active courses"}}},
	}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}

	tests := []struct {
		name   string
		schema *ConfigurationSchema
	}{
		{"duplicate key", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "title", Type: ConfigurationText, Label: "One"}, {Key: "title", Type: ConfigurationText, Label: "Two"}}}},
		{"dangerous key", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "constructor", Type: ConfigurationText, Label: "Title"}}}},
		{"unsupported type", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "title", Type: "HTML", Label: "Title"}}}},
		{"too many fields", &ConfigurationSchema{Fields: func() []ConfigurationField {
			fields := make([]ConfigurationField, MaxConfigurationFields+1)
			for index := range fields {
				fields[index] = ConfigurationField{Key: "field" + strings.Repeat("x", index+1), Type: ConfigurationText, Label: "Field"}
			}
			return fields
		}()}},
		{"too many options", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "scope", Type: ConfigurationSingleSelect, Label: "Scope", Options: func() []ConfigurationOption {
			options := make([]ConfigurationOption, MaxConfigurationOptions+1)
			for index := range options {
				options[index] = ConfigurationOption{Value: strings.Repeat("x", index+1), Label: "Option"}
			}
			return options
		}()}}}},
		{"wrong default type", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "count", Type: ConfigurationInteger, Label: "Count", Default: json.RawMessage(`"five"`)}}}},
		{"default outside bounds", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "count", Type: ConfigurationInteger, Label: "Count", Default: json.RawMessage(`7`), Max: floatPointer(5)}}}},
		{"select default not listed", &ConfigurationSchema{Fields: []ConfigurationField{{Key: "scope", Type: ConfigurationSingleSelect, Label: "Scope", Default: json.RawMessage(`"missing"`), Options: []ConfigurationOption{{Value: "all", Label: "All"}}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !errors.Is(test.schema.Validate(), ErrInvalidConfigurationSchema) {
				t.Fatal("invalid schema accepted")
			}
		})
	}
}

func TestEntrypointNormalizeConfiguration(t *testing.T) {
	entrypoint := Entrypoint{Configuration: &ConfigurationSchema{Fields: []ConfigurationField{
		{Key: "title", Type: ConfigurationText, Label: "Title", Required: true, MinLength: intPointer(2), MaxLength: intPointer(8)},
		{Key: "count", Type: ConfigurationInteger, Label: "Count", Min: floatPointer(1), Max: floatPointer(5)},
		{Key: "ratio", Type: ConfigurationNumber, Label: "Ratio"},
		{Key: "visible", Type: ConfigurationBoolean, Label: "Visible", Default: json.RawMessage(`true`)},
		{Key: "scope", Type: ConfigurationSingleSelect, Label: "Scope", Options: []ConfigurationOption{{Value: "all", Label: "All"}}},
	}}}
	normalized, err := entrypoint.NormalizeConfiguration(json.RawMessage(`{"title":"Recent","count":3,"ratio":1.5,"scope":"all"}`))
	if err != nil || string(normalized) != `{"count":3,"ratio":1.5,"scope":"all","title":"Recent","visible":true}` {
		t.Fatalf("normalization failed: %s %v", normalized, err)
	}

	for name, raw := range map[string]string{
		"unknown key":      `{"title":"Recent","other":true}`,
		"wrong type":       `{"title":3}`,
		"missing required": `{}`,
		"string bounds":    `{"title":"x"}`,
		"numeric bounds":   `{"title":"Recent","count":6}`,
		"integer fraction": `{"title":"Recent","count":1.5}`,
		"invalid select":   `{"title":"Recent","scope":"active"}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := entrypoint.NormalizeConfiguration(json.RawMessage(raw)); !errors.Is(err, ErrInvalidWidgetConfiguration) {
				t.Fatalf("invalid configuration accepted: %v", err)
			}
		})
	}

	plain := Entrypoint{}
	if normalized, err := plain.NormalizeConfiguration(json.RawMessage(`{}`)); err != nil || string(normalized) != `{}` {
		t.Fatalf("empty no-schema configuration rejected: %s %v", normalized, err)
	}
	if _, err := plain.NormalizeConfiguration(json.RawMessage(`{"advanced":true}`)); !errors.Is(err, ErrInvalidWidgetConfiguration) {
		t.Fatal("arbitrary JSON bypassed absent schema")
	}
}
