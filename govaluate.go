// Package fxmdgovaluate applies native Govaluate rules to FXMacroData JSON.
// It performs no network I/O and accepts no credentials or application objects.
package fxmdgovaluate

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/Knetic/govaluate"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxResponseBytes  = 4 << 20
	MaxRuleBytes      = 4096
	MaxJSONDepth      = 64
	MaxJSONValues     = 50000
	MaxRuleTokens     = 512
	MaxViewBytes      = 8 << 20
	MaxConstantsBytes = 64 << 10
)

// Result separates a computed rule value from an independent, unchanged response.
type Result struct {
	Value any
	Raw   json.RawMessage
}

// Evaluate serializes a caller-owned SDK response. EvaluateJSON retains original
// HTTP bytes and unknown fields which a typed SDK model might otherwise omit.
func Evaluate(data any, rule string) (Result, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Result{}, errors.New("response is not JSON-serializable")
	}
	return EvaluateJSON(raw, rule)
}

// EvaluateJSON exposes only JSON Pointer functions to a native expression.
// Rules are reviewed application configuration, not hostile tenant code.
func EvaluateJSON(raw []byte, rule string) (Result, error) {
	return EvaluateJSONWithConstants(raw, rule, nil)
}

// EvaluateJSONWithConstants supplies explicit JSON scalar variables without
// exposing Go objects or implicitly converting date-looking string literals.
func EvaluateJSONWithConstants(raw []byte, rule string, constantsJSON []byte) (result Result, err error) {
	defer func() {
		if recover() != nil {
			result = Result{}
			err = errors.New("rule evaluation failed")
		}
	}()
	if len(raw) == 0 || len(raw) > MaxResponseBytes || !utf8.Valid(raw) {
		return Result{}, errors.New("response size or encoding is invalid")
	}
	if len(rule) == 0 || len(rule) > MaxRuleBytes || !utf8.ValidString(rule) {
		return Result{}, errors.New("rule size or encoding is invalid")
	}
	owned := append(json.RawMessage(nil), raw...)
	dec := json.NewDecoder(bytes.NewReader(owned))
	dec.UseNumber()
	remaining := MaxJSONValues
	data, err := decode(dec, 0, &remaining)
	if err != nil {
		return Result{}, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return Result{}, errors.New("exactly one JSON value is required")
	}
	budget := MaxViewBytes
	take := func(size int) error {
		budget -= size
		if budget < 0 {
			return errors.New("rule view budget exceeded")
		}
		return nil
	}
	parameters := map[string]interface{}{}
	if len(constantsJSON) > 0 {
		if len(constantsJSON) > MaxConstantsBytes || !utf8.Valid(constantsJSON) {
			return Result{}, errors.New("constant size or encoding is invalid")
		}
		reader := json.NewDecoder(bytes.NewReader(constantsJSON))
		reader.UseNumber()
		remaining := MaxJSONValues
		value, err := decode(reader, 0, &remaining)
		if err != nil {
			return Result{}, errors.New("invalid JSON constants")
		}
		if _, err := reader.Token(); err != io.EOF {
			return Result{}, errors.New("exactly one constant object is required")
		}
		object, ok := value.(map[string]any)
		if !ok {
			return Result{}, errors.New("constants must be a JSON object")
		}
		for key, value := range object {
			if !constantName(key) {
				return Result{}, errors.New("invalid or reserved constant name")
			}
			scalar, err := nativeScalar(value, take)
			if err != nil {
				return Result{}, errors.New("constants must be supported JSON scalars")
			}
			parameters[key] = scalar
		}
	}
	selectValue := func(args []interface{}) (any, bool, error) {
		if len(args) != 1 {
			return nil, false, errors.New("one JSON Pointer string is required")
		}
		pointer, ok := args[0].(string)
		if !ok {
			return nil, false, errors.New("a JSON Pointer string is required")
		}
		return lookup(data, pointer)
	}
	functions := map[string]govaluate.ExpressionFunction{
		"exists": func(args ...interface{}) (interface{}, error) { _, found, err := selectValue(args); return found, err },
		"field": func(args ...interface{}) (interface{}, error) {
			value, found, err := selectValue(args)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, errors.New("JSON field is unavailable")
			}
			return nativeScalar(value, take)
		},
		"json": func(args ...interface{}) (interface{}, error) {
			value, found, err := selectValue(args)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, errors.New("JSON field is unavailable")
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, errors.New("JSON view failed")
			}
			if err := take(len(encoded)); err != nil {
				return nil, err
			}
			return string(encoded), nil
		},
		"length": func(args ...interface{}) (interface{}, error) {
			value, found, err := selectValue(args)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, errors.New("JSON field is unavailable")
			}
			switch value := value.(type) {
			case []any:
				return float64(len(value)), nil
			case map[string]any:
				return float64(len(value)), nil
			case string:
				return float64(utf8.RuneCountInString(value)), nil
			default:
				return nil, errors.New("length requires an array, object or string")
			}
		},
	}
	expression, err := govaluate.NewEvaluableExpressionWithFunctions(rule, functions)
	if err != nil {
		return Result{}, errors.New("invalid native rule")
	}
	tokens := expression.Tokens()
	if len(tokens) > MaxRuleTokens {
		return Result{}, errors.New("rule token limit exceeded")
	}
	for _, token := range tokens {
		if token.Kind == govaluate.TIME {
			return Result{}, errors.New("implicit date literals are disabled; compare JSON text instead")
		}
		if token.Kind == govaluate.VARIABLE {
			if _, ok := parameters[token.Value.(string)]; !ok {
				return Result{}, errors.New("only explicit JSON constants are exposed")
			}
		}
		if token.Kind == govaluate.NUMERIC {
			if n, ok := token.Value.(float64); ok && (math.IsInf(n, 0) || math.IsNaN(n) || math.Abs(n) > 9007199254740991) {
				return Result{}, errors.New("numeric literal is outside the native safe range")
			}
		}
	}
	expression.ChecksTypes = true
	value, err := expression.Evaluate(parameters)
	if err != nil {
		return Result{}, errors.New("native rule evaluation failed")
	}
	if number, ok := value.(float64); ok && (math.IsInf(number, 0) || math.IsNaN(number)) {
		return Result{}, errors.New("rule result must be finite")
	}
	if text, ok := value.(string); ok && len(text) > MaxViewBytes {
		return Result{}, errors.New("rule result exceeds the view budget")
	}
	return Result{Value: value, Raw: owned}, nil
}
func nativeScalar(value any, take func(int) error) (any, error) {
	switch value := value.(type) {
	case json.Number:
		number, err := strconv.ParseFloat(value.String(), 64)
		if err != nil || math.IsInf(number, 0) || math.IsNaN(number) || math.Abs(number) > 9007199254740991 {
			return nil, errors.New("number is outside the native safe range; use json for exact text")
		}
		return number, nil
	case string:
		if err := take(len(value)); err != nil {
			return nil, err
		}
		return value, nil
	case bool, nil:
		return value, nil
	default:
		return nil, errors.New("a native scalar value is required")
	}
}

// lookup follows RFC 6901 pointers. Presence is separate from a JSON null value.
func lookup(root any, pointer string) (any, bool, error) {
	if pointer == "" {
		return root, true, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, false, errors.New("invalid JSON Pointer")
	}
	current := root
	for _, part := range strings.Split(pointer[1:], "/") {
		var name strings.Builder
		for i := 0; i < len(part); i++ {
			if part[i] != '~' {
				name.WriteByte(part[i])
				continue
			}
			i++
			if i >= len(part) {
				return nil, false, errors.New("invalid JSON Pointer escape")
			}
			switch part[i] {
			case '0':
				name.WriteByte('~')
			case '1':
				name.WriteByte('/')
			default:
				return nil, false, errors.New("invalid JSON Pointer escape")
			}
		}
		key := name.String()
		switch value := current.(type) {
		case map[string]any:
			var found bool
			current, found = value[key]
			if !found {
				return nil, false, nil
			}
		case []any:
			if key == "" || (len(key) > 1 && key[0] == '0') {
				return nil, false, errors.New("invalid JSON array index")
			}
			for _, c := range key {
				if c < '0' || c > '9' {
					return nil, false, errors.New("invalid JSON array index")
				}
			}
			index, err := strconv.ParseUint(key, 10, 64)
			if err != nil || index >= uint64(len(value)) {
				return nil, false, nil
			}
			current = value[int(index)]
		default:
			return nil, false, nil
		}
	}
	return current, true, nil
}

func decode(dec *json.Decoder, depth int, remaining *int) (any, error) {
	if depth > MaxJSONDepth {
		return nil, errors.New("nesting limit exceeded")
	}
	*remaining--
	if *remaining < 0 {
		return nil, errors.New("value limit exceeded")
	}
	token, err := dec.Token()
	if err != nil {
		return nil, errors.New("malformed JSON")
	}
	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			object := map[string]any{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, errors.New("malformed object")
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("non-string object key")
				}
				if _, exists := object[name]; exists {
					return nil, errors.New("duplicate object key")
				}
				item, err := decode(dec, depth+1, remaining)
				if err != nil {
					return nil, err
				}
				object[name] = item
			}
			close, err := dec.Token()
			if err != nil || close != json.Delim('}') {
				return nil, errors.New("malformed object")
			}
			return object, nil
		case '[':
			array := []any{}
			for dec.More() {
				item, err := decode(dec, depth+1, remaining)
				if err != nil {
					return nil, err
				}
				array = append(array, item)
			}
			close, err := dec.Token()
			if err != nil || close != json.Delim(']') {
				return nil, errors.New("malformed array")
			}
			return array, nil
		}
		return nil, errors.New("unexpected delimiter")
	case json.Number:
		return value, nil
	case string, bool, nil:
		return token, nil
	}
	return nil, errors.New("unsupported JSON token")
}

func constantName(name string) bool {
	if name == "" {
		return false
	}
	switch name {
	case "field", "exists", "json", "length", "true", "false", "in":
		return false
	}
	for i, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}
