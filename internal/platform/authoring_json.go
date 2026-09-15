package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"reflect"
	"strings"
)

// All Authoring mutations share bounded, unambiguous JSON parsing. The token
// pass rejects duplicate members (including canonical content payloads) before
// typed decoding can silently choose the last value.
func decodeAuthoringBody(w http.ResponseWriter, r *http.Request, limit int64, value any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("invalid content type")
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueAuthoringJSON(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	if err := authoringJSONFields(data, reflect.TypeOf(value)); err != nil {
		return err
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func uniqueAuthoringJSON(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("JSON nesting too deep")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok || seen[key] {
				return errors.New("duplicate JSON member")
			}
			seen[key] = true
			if err := uniqueAuthoringJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueAuthoringJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}

// Enforce exact request field names and non-null native values. Custom domain
// decoders/optional PATCH fields retain their own null and semantic validation.
func authoringJSONFields(data []byte, shape reflect.Type) error {
	for shape.Kind() == reflect.Pointer {
		shape = shape.Elem()
	}
	if shape.Kind() != reflect.Struct {
		return nil
	}
	unmarshaler := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()
	if reflect.PointerTo(shape).Implements(unmarshaler) {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("JSON object required")
	}
	allowed := make(map[string]reflect.Type)
	for i := 0; i < shape.NumField(); i++ {
		field := shape.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name != "" && name != "-" {
			allowed[name] = field.Type
		}
	}
	for name, raw := range fields {
		field, found := allowed[name]
		if !found {
			return errors.New("unknown JSON member")
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			if field.Kind() != reflect.Pointer && !reflect.PointerTo(field).Implements(unmarshaler) {
				return errors.New("invalid null JSON member")
			}
			continue
		}
		for field.Kind() == reflect.Pointer {
			field = field.Elem()
		}
		if field.Kind() == reflect.Struct {
			if err := authoringJSONFields(raw, field); err != nil {
				return err
			}
		}
		if field.Kind() == reflect.Slice && field.Elem().Kind() == reflect.Struct {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err != nil {
				return err
			}
			for _, item := range items {
				if err := authoringJSONFields(item, field.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateAuthoringLicenseJSON(data []byte) error {
	type fields authoringLicenseRequest
	return authoringJSONFields(data, reflect.TypeOf(fields{}))
}
