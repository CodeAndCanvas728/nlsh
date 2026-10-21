package plan

import (
	"encoding/json"
	"testing"
)

func TestSchemaIsValidJSON(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(Schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
}

func TestSchemaMatchesThePlanShape(t *testing.T) {
	schema, err := SchemaObject()
	if err != nil {
		t.Fatalf("SchemaObject: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties: %v", schema)
	}
	for _, field := range []string{"commands", "explanation", "caveat"} {
		if _, ok := properties[field]; !ok {
			t.Errorf("schema is missing the %q property", field)
		}
	}

	required, ok := schema["required"].([]any)
	if !ok {
		t.Fatalf("schema has no required list: %v", schema)
	}
	if len(required) != 3 {
		t.Errorf("schema requires %d fields, want 3", len(required))
	}
}

func TestDecode(t *testing.T) {
	const raw = `{"commands":[{"command":"ls -l","risk":"safe"}],"explanation":"List.","caveat":""}`
	p, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(p.Commands) != 1 {
		t.Fatalf("got %d commands, want 1", len(p.Commands))
	}
	if p.Commands[0].Command != "ls -l" {
		t.Errorf("command = %q, want %q", p.Commands[0].Command, "ls -l")
	}
	if p.Explanation != "List." {
		t.Errorf("explanation = %q", p.Explanation)
	}
}

func TestDecodeRejectsRubbish(t *testing.T) {
	if _, err := Decode("not json at all"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		ok   bool
	}{
		{`{"a":1}`, `{"a":1}`, true},
		{"Here you go:\n{\"a\":1}\nEnjoy.", `{"a":1}`, true},
		{"no json here", "", false},
		{"{unclosed", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, ok := ExtractJSON(tc.raw)
			if got != tc.want || ok != tc.ok {
				t.Errorf("ExtractJSON(%q) = %q, %v; want %q, %v", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestRiskValid(t *testing.T) {
	for _, r := range []Risk{RiskSafe, RiskCareful, RiskDestructive} {
		if !r.Valid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if Risk("unknown").Valid() {
		t.Error("unknown should not be valid")
	}
}
