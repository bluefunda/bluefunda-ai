package jsonschema

import (
	"encoding/json"
	"strings"
	"testing"
)

const schema = `{
	"type": "object",
	"properties": {
		"name":  {"type": "string", "minLength": 1, "pattern": "^[a-z]+$"},
		"count": {"type": "integer", "minimum": 0, "maximum": 10},
		"score": {"type": "number"},
		"tags":  {"type": "array", "items": {"type": "string", "enum": ["a", "b"]}, "maxItems": 2},
		"kind":  {"const": "x"},
		"maybe": {"type": ["string", "null"]},
		"either": {"anyOf": [{"type": "string"}, {"type": "integer"}]}
	},
	"required": ["name", "count"],
	"additionalProperties": false
}`

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestValidate(t *testing.T) {
	s, err := Compile([]byte(schema))
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.Validate(decode(t, `{"name":"ok","count":3,"score":1,"tags":["a"],"kind":"x","maybe":null,"either":2}`)); errs != nil {
		t.Errorf("valid value rejected: %v", errs)
	}
	cases := map[string]string{
		`{"count":1}`:                                  `$: missing required property "name"`,
		`{"name":"ok","count":1.5}`:                    `$.count: expected integer, got number`,
		`{"name":"ok","count":11}`:                     `$.count: must be <= 10`,
		`{"name":"OK","count":1}`:                      `$.name: must match pattern`,
		`{"name":"ok","count":1,"extra":1}`:            `unexpected property "extra"`,
		`{"name":"ok","count":1,"tags":["c"]}`:         `$.tags[0]: must be one of ["a","b"]`,
		`{"name":"ok","count":1,"tags":["a","a","b"]}`: `allows at most 2 items`,
		`{"name":"ok","count":1,"kind":"y"}`:           `$.kind: must be "x"`,
		`{"name":"ok","count":1,"either":true}`:        `matches none of anyOf`,
		`[]`:                                           `$: expected object, got array`,
	}
	for in, want := range cases {
		errs := s.Validate(decode(t, in))
		if !strings.Contains(strings.Join(errs, "\n"), want) {
			t.Errorf("%s: errors %q, want one containing %q", in, errs, want)
		}
	}
}

func TestCompileRejects(t *testing.T) {
	for _, bad := range []string{`not json`, `[]`, `{"$ref":"#/x"}`, `{"properties":{"a":{"pattern":"("}}}`} {
		if _, err := Compile([]byte(bad)); err == nil {
			t.Errorf("compiled %s", bad)
		}
	}
}

func TestExtract(t *testing.T) {
	for in, want := range map[string]string{
		`{"a":1}`: `{"a":1}`,
		"Here you go:\n```json\n{\"a\":1}\n```\n": `{"a":1}`,
		`The answer is {"a":1}. Done.`:            `{"a":1}`,
		`[1,2]`:                                   `[1,2]`,
	} {
		v, err := Extract(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got, _ := json.Marshal(v); string(got) != want {
			t.Errorf("%q → %s, want %s", in, got, want)
		}
	}
	if _, err := Extract("no json here"); err == nil {
		t.Error("expected an error")
	}
}
