package plugins

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ConfigurationFieldType string

const (
	ConfigurationText                 ConfigurationFieldType = "TEXT"
	ConfigurationTextarea             ConfigurationFieldType = "TEXTAREA"
	ConfigurationInteger              ConfigurationFieldType = "INTEGER"
	ConfigurationNumber               ConfigurationFieldType = "NUMBER"
	ConfigurationBoolean              ConfigurationFieldType = "BOOLEAN"
	ConfigurationSingleSelect         ConfigurationFieldType = "SINGLE_SELECT"
	MaxConfigurationFields                                   = 32
	MaxConfigurationOptions                                  = 50
	MaxConfigurationKeyLength                                = 64
	MaxConfigurationLabelLength                              = 120
	MaxConfigurationDescriptionLength                        = 500
	MaxConfigurationOptionLength                             = 120
	MaxWidgetConfigurationBytes                              = 16 << 10
)

var (
	configurationKeyPattern       = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	ErrInvalidConfigurationSchema = errors.New("invalid widget configuration schema")
	ErrInvalidWidgetConfiguration = errors.New("invalid widget configuration")
)

type ConfigurationOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ConfigurationField struct {
	Key         string                 `json:"key"`
	Type        ConfigurationFieldType `json:"type"`
	Label       string                 `json:"label"`
	Description string                 `json:"description,omitempty"`
	Required    bool                   `json:"required"`
	Default     json.RawMessage        `json:"default,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Min         *float64               `json:"min,omitempty"`
	Max         *float64               `json:"max,omitempty"`
	Options     []ConfigurationOption  `json:"options,omitempty"`
}

type ConfigurationSchema struct {
	Fields []ConfigurationField `json:"fields"`
}

func (s *ConfigurationSchema) Validate() error {
	if s == nil {
		return nil
	}
	if len(s.Fields) > MaxConfigurationFields {
		return ErrInvalidConfigurationSchema
	}
	seen := map[string]bool{}
	for _, field := range s.Fields {
		if !configurationKeyPattern.MatchString(field.Key) || len(field.Key) > MaxConfigurationKeyLength || field.Key == "constructor" || field.Key == "prototype" || field.Key == "__proto__" || seen[field.Key] || strings.TrimSpace(field.Label) == "" || len(field.Label) > MaxConfigurationLabelLength || len(field.Description) > MaxConfigurationDescriptionLength {
			return ErrInvalidConfigurationSchema
		}
		seen[field.Key] = true
		if err := field.validateMetadata(); err != nil {
			return err
		}
		if len(field.Default) > 0 {
			if err := field.validateValue(field.Default); err != nil {
				return ErrInvalidConfigurationSchema
			}
		}
	}
	return nil
}

func (f ConfigurationField) validateMetadata() error {
	text := f.Type == ConfigurationText || f.Type == ConfigurationTextarea
	numeric := f.Type == ConfigurationInteger || f.Type == ConfigurationNumber
	if !text && (f.MinLength != nil || f.MaxLength != nil) || !numeric && (f.Min != nil || f.Max != nil) || f.Type != ConfigurationSingleSelect && len(f.Options) > 0 {
		return ErrInvalidConfigurationSchema
	}
	if text && ((f.MinLength != nil && (*f.MinLength < 0 || *f.MinLength > MaxWidgetConfigurationBytes)) || (f.MaxLength != nil && (*f.MaxLength < 0 || *f.MaxLength > MaxWidgetConfigurationBytes)) || (f.MinLength != nil && f.MaxLength != nil && *f.MinLength > *f.MaxLength)) {
		return ErrInvalidConfigurationSchema
	}
	if numeric && f.Min != nil && f.Max != nil && *f.Min > *f.Max {
		return ErrInvalidConfigurationSchema
	}
	if f.Type == ConfigurationSingleSelect {
		if len(f.Options) == 0 || len(f.Options) > MaxConfigurationOptions {
			return ErrInvalidConfigurationSchema
		}
		seen := map[string]bool{}
		for _, option := range f.Options {
			if strings.TrimSpace(option.Value) == "" || strings.TrimSpace(option.Label) == "" || len(option.Value) > MaxConfigurationOptionLength || len(option.Label) > MaxConfigurationOptionLength || seen[option.Value] {
				return ErrInvalidConfigurationSchema
			}
			seen[option.Value] = true
		}
	}
	switch f.Type {
	case ConfigurationText, ConfigurationTextarea, ConfigurationInteger, ConfigurationNumber, ConfigurationBoolean, ConfigurationSingleSelect:
		return nil
	default:
		return ErrInvalidConfigurationSchema
	}
}

func (e Entrypoint) NormalizeConfiguration(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > MaxWidgetConfigurationBytes || e.Configuration.Validate() != nil {
		return nil, ErrInvalidWidgetConfiguration
	}
	var values map[string]json.RawMessage
	if strictDecode(raw, &values) != nil || values == nil {
		return nil, ErrInvalidWidgetConfiguration
	}
	fields := map[string]ConfigurationField{}
	if e.Configuration != nil {
		for _, field := range e.Configuration.Fields {
			fields[field.Key] = field
		}
	}
	for key := range values {
		if _, ok := fields[key]; !ok {
			return nil, ErrInvalidWidgetConfiguration
		}
	}
	for key, field := range fields {
		value, ok := values[key]
		if !ok && len(field.Default) > 0 {
			values[key] = append(json.RawMessage(nil), field.Default...)
			value, ok = values[key]
		}
		if !ok {
			if field.Required {
				return nil, ErrInvalidWidgetConfiguration
			}
			continue
		}
		if field.validateValue(value) != nil {
			return nil, ErrInvalidWidgetConfiguration
		}
	}
	normalized, err := json.Marshal(values)
	if err != nil || len(normalized) > MaxWidgetConfigurationBytes {
		return nil, ErrInvalidWidgetConfiguration
	}
	return normalized, nil
}

func (f ConfigurationField) validateValue(raw json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return ErrInvalidWidgetConfiguration
	}
	switch f.Type {
	case ConfigurationText, ConfigurationTextarea:
		text, ok := value.(string)
		length := utf8.RuneCountInString(text)
		if !ok || f.MinLength != nil && length < *f.MinLength || f.MaxLength != nil && length > *f.MaxLength {
			return ErrInvalidWidgetConfiguration
		}
	case ConfigurationInteger, ConfigurationNumber:
		number, ok := value.(json.Number)
		parsed, err := number.Float64()
		if !ok || err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) || f.Type == ConfigurationInteger && math.Trunc(parsed) != parsed || f.Min != nil && parsed < *f.Min || f.Max != nil && parsed > *f.Max {
			return ErrInvalidWidgetConfiguration
		}
	case ConfigurationBoolean:
		if _, ok := value.(bool); !ok {
			return ErrInvalidWidgetConfiguration
		}
	case ConfigurationSingleSelect:
		selected, ok := value.(string)
		if !ok {
			return ErrInvalidWidgetConfiguration
		}
		for _, option := range f.Options {
			if option.Value == selected {
				return nil
			}
		}
		return ErrInvalidWidgetConfiguration
	default:
		return ErrInvalidWidgetConfiguration
	}
	return nil
}
