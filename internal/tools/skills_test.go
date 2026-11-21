package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, dir, md string) string {
	t.Helper()
	d := filepath.Join(root, dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLoadSkills(t *testing.T) {
	user, proj := t.TempDir(), t.TempDir()
	writeSkill(t, user, "release", "---\nname: release\ndescription: user version\n---\nUser steps.")
	writeSkill(t, proj, "release", "---\nname: release\ndescription: project version\n---\nProject steps.")
	writeSkill(t, proj, "pdf-forms", "---\ndescription: Fill PDF forms\n---\n\n# Steps\nUse forms.py.\n")
	writeSkill(t, proj, "broken", "no frontmatter")
	writeSkill(t, proj, "nodesc", "---\nname: nodesc\n---\nx")

	skills, errs := LoadSkills(user, proj, "")
	if len(errs) != 2 {
		t.Errorf("errs = %v, want broken and nodesc", errs)
	}
	if len(skills) != 2 || skills[0].Name != "pdf-forms" || skills[1].Name != "release" {
		t.Fatalf("skills = %+v", skills)
	}
	if skills[0].Body != "# Steps\nUse forms.py." || !filepath.IsAbs(skills[0].Dir) {
		t.Errorf("pdf-forms = %+v (name from the folder, body trimmed, absolute dir)", skills[0])
	}
	if skills[1].Description != "project version" {
		t.Errorf("the project skill should win: %q", skills[1].Description)
	}
}

func TestSkillTool(t *testing.T) {
	root := t.TempDir()
	dir := writeSkill(t, root, "pdf", "---\ndescription: Fill PDF forms\n---\nRun forms.py.")
	if err := os.WriteFile(filepath.Join(dir, "forms.py"), []byte("print(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("s3cret"), 0o644); err != nil {
		t.Fatal(err)
	}
	skills, _ := LoadSkills(root)

	SetSkills(nil)
	if len(skillSchemas()) != 0 {
		t.Error("no skills: the skill tool should not be offered")
	}
	SetSkills(skills)
	t.Cleanup(func() { SetSkills(nil) })

	schemas := skillSchemas()
	if len(schemas) != 1 || !strings.Contains(schemas[0].Function.Description, "- pdf: Fill PDF forms") {
		t.Fatalf("schema = %+v", schemas)
	}
	var params map[string]any
	if err := json.Unmarshal(schemas[0].Function.Parameters, &params); err != nil {
		t.Fatalf("parameters are not valid JSON: %v", err)
	}

	if out, err := Execute("skill", `{"name":"pdf"}`); err != nil || !strings.Contains(out, "Run forms.py.") {
		t.Errorf("SKILL.md: %q, %v", out, err)
	}
	if out, err := Execute("skill", `{"name":"pdf","file":"forms.py"}`); err != nil || out != "print(1)" {
		t.Errorf("supporting file: %q, %v", out, err)
	}
	if out, err := Execute("skill", `{"name":"pdf","file":"../secret"}`); err == nil || strings.Contains(out, "s3cret") {
		t.Errorf("read outside the skill folder: %q, %v", out, err)
	}
	if _, err := Execute("skill", `{"name":"nope"}`); err == nil {
		t.Error("unknown skill should fail")
	}
	if !IsReadOnlyTool("skill") {
		t.Error("loading a skill reads files only; it should never ask for approval")
	}
}
