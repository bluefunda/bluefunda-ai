// Package jsonschema validates JSON values against the subset of JSON Schema
// that structured output uses: type, properties, required,
// additionalProperties, items, enum, const, minItems/maxItems,
// minLength/maxLength, minimum/maximum, pattern, anyOf, oneOf and allOf.
// Other keywords are ignored; $ref is rejected when compiling, since a
// schema that relies on it can't be checked here.
package jsonschema

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Schema is a compiled schema.
type Schema struct {
	raw any
}

// Compile parses a schema and rejects one this package can't check.
func Compile(data []byte) (*Schema, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("schema is not valid JSON: %w", err)
	}
	if _, ok := raw.(map[string]any); !ok {
		return nil, fmt.Errorf("schema must be a JSON object")
	}
	if err := check(raw); err != nil {
		return nil, err
	}
	return &Schema{raw: raw}, nil
}

// check walks the schema for $ref and bad patterns.
func check(node any) error {
	switch n := node.(type) {
	case map[string]any:
		if _, ok := n["$ref"]; ok {
			return fmt.Errorf("$ref is not supported; inline the referenced schema")
		}
		if p, ok := n["pattern"].(string); ok {
			if _, err := regexp.Compile(p); err != nil {
				return fmt.Errorf("pattern %q: %w", p, err)
			}
		}
		for _, v := range n {
			if err := check(v); err != nil {
				return err
			}
		}
	case []any:
		for _, v := range n {
			if err := check(v); err != nil {
				return err
			}
		}
	}
	return nil
}

// String returns the schema as compact JSON.
func (s *Schema) String() string {
	b, _ := json.Marshal(s.raw)
	return string(b)
}

// Validate checks a decoded JSON value (as from json.Unmarshal into any)
// and returns every problem found, each prefixed with its location ("$" is
// the root), or nil if the value is valid.
func (s *Schema) Validate(v any) []string {
	var errs []string
	validate(s.raw, v, "$", &errs)
	return errs
}

func validate(schema, v any, at string, errs *[]string) {
	sch, ok := schema.(map[string]any)
	if !ok {
		return // true / {} and unknown forms accept anything
	}
	fail := func(format string, args ...any) {
		*errs = append(*errs, at+": "+fmt.Sprintf(format, args...))
	}
	if t, ok := sch["type"]; ok && !typeMatches(t, v) {
		fail("expected %s, got %s", typeList(t), typeOf(v))
		return
	}
	if enum, ok := sch["enum"].([]any); ok && !containsJSON(enum, v) {
		fail("must be one of %s", compact(enum))
	}
	if c, ok := sch["const"]; ok && !equalJSON(c, v) {
		fail("must be %s", compact(c))
	}
	for _, kw := range []string{"allOf", "anyOf", "oneOf"} {
		subs, ok := sch[kw].([]any)
		if !ok {
			continue
		}
		passed := 0
		var first []string
		for _, sub := range subs {
			var e []string
			validate(sub, v, at, &e)
			if len(e) == 0 {
				passed++
			} else if first == nil {
				first = e
			}
		}
		switch {
		case kw == "allOf" && passed != len(subs):
			*errs = append(*errs, first...)
		case kw == "anyOf" && passed == 0:
			fail("matches none of anyOf")
		case kw == "oneOf" && passed != 1:
			fail("must match exactly one of oneOf, matches %d", passed)
		}
	}

	switch val := v.(type) {
	case map[string]any:
		props, _ := sch["properties"].(map[string]any)
		if req, ok := sch["required"].([]any); ok {
			for _, r := range req {
				if name, ok := r.(string); ok {
					if _, present := val[name]; !present {
						fail("missing required property %q", name)
					}
				}
			}
		}
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k]; ok {
				validate(ps, val[k], at+"."+k, errs)
				continue
			}
			switch ap := sch["additionalProperties"].(type) {
			case bool:
				if !ap {
					fail("unexpected property %q", k)
				}
			case map[string]any:
				validate(ap, val[k], at+"."+k, errs)
			}
		}
	case []any:
		if n, ok := num(sch["minItems"]); ok && float64(len(val)) < n {
			fail("needs at least %v items, has %d", n, len(val))
		}
		if n, ok := num(sch["maxItems"]); ok && float64(len(val)) > n {
			fail("allows at most %v items, has %d", n, len(val))
		}
		if items, ok := sch["items"]; ok {
			for i, item := range val {
				validate(items, item, fmt.Sprintf("%s[%d]", at, i), errs)
			}
		}
	case string:
		n := float64(utf8.RuneCountInString(val))
		if m, ok := num(sch["minLength"]); ok && n < m {
			fail("must be at least %v characters", m)
		}
		if m, ok := num(sch["maxLength"]); ok && n > m {
			fail("must be at most %v characters", m)
		}
		if p, ok := sch["pattern"].(string); ok {
			if re, err := regexp.Compile(p); err == nil && !re.MatchString(val) {
				fail("must match pattern %q", p)
			}
		}
	case float64:
		if m, ok := num(sch["minimum"]); ok && val < m {
			fail("must be >= %v", m)
		}
		if m, ok := num(sch["maximum"]); ok && val > m {
			fail("must be <= %v", m)
		}
	}
}

func num(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

func typeOf(v any) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		if val == math.Trunc(val) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

func typeMatches(t, v any) bool {
	got := typeOf(v)
	match := func(want string) bool {
		return want == got || (want == "number" && got == "integer")
	}
	switch tt := t.(type) {
	case string:
		return match(tt)
	case []any:
		for _, x := range tt {
			if s, ok := x.(string); ok && match(s) {
				return true
			}
		}
		return false
	}
	return true
}

func typeList(t any) string {
	if list, ok := t.([]any); ok {
		parts := make([]string, len(list))
		for i, x := range list {
			parts[i] = fmt.Sprint(x)
		}
		return strings.Join(parts, " or ")
	}
	return fmt.Sprint(t)
}

func equalJSON(a, b any) bool { return compact(a) == compact(b) }

func containsJSON(list []any, v any) bool {
	for _, x := range list {
		if equalJSON(x, v) {
			return true
		}
	}
	return false
}

func compact(v any) string {
	b, _ := json.Marshal(v) // map keys are sorted, so equal values match
	return string(b)
}

// Extract finds the JSON value in a model's answer: the whole text, or the
// contents of a ```json fence, or the outermost {...} or [...] in it.
func Extract(text string) (any, error) {
	text = strings.TrimSpace(text)
	candidates := []string{text}
	if i := strings.Index(text, "```"); i >= 0 {
		rest := text[i+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			candidates = append(candidates, rest[:j])
		}
	}
	for _, pair := range [][2]string{{"{", "}"}, {"[", "]"}} {
		if i, j := strings.Index(text, pair[0]), strings.LastIndex(text, pair[1]); i >= 0 && j > i {
			candidates = append(candidates, text[i:j+1])
		}
	}
	for _, c := range candidates {
		var v any
		if json.Unmarshal([]byte(strings.TrimSpace(c)), &v) == nil {
			return v, nil
		}
	}
	return nil, fmt.Errorf("the answer contains no JSON value")
}
