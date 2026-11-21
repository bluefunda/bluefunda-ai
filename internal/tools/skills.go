package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Skill is a folder of instructions (and any supporting files) the model
// loads when a task calls for it, like Claude Code's skills: the model sees
// every skill's name and description, and reads the full SKILL.md with the
// skill tool only when it needs it.
type Skill struct {
	Name        string
	Description string
	Dir         string // the skill's folder
	Body        string // SKILL.md without its frontmatter
}

// maxSkillFile caps a supporting file the skill tool returns.
const maxSkillFile = 256 << 10

// ParseSkill reads dir/SKILL.md. name defaults to the folder's name;
// description is required.
func ParseSkill(dir string) (Skill, error) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	path := filepath.Join(dir, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	head, body, err := splitFrontmatter(path, data)
	if err != nil {
		return Skill{}, err
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(head, &fm); err != nil {
		return Skill{}, fmt.Errorf("%s: frontmatter: %w", path, err)
	}
	if fm.Name == "" {
		fm.Name = filepath.Base(dir)
	}
	if !agentNameRE.MatchString(fm.Name) {
		return Skill{}, fmt.Errorf("%s: name %q must be lowercase letters, digits, - or _", path, fm.Name)
	}
	if strings.TrimSpace(fm.Description) == "" {
		return Skill{}, fmt.Errorf("%s: description is required", path)
	}
	return Skill{Name: fm.Name, Description: strings.TrimSpace(fm.Description), Dir: dir, Body: body}, nil
}

// LoadSkills reads <dir>/*/SKILL.md for each dir in order; a later dir wins
// on a name clash, so pass the user directory before the project one.
// Folders that fail to parse are skipped and reported.
func LoadSkills(dirs ...string) (skills []Skill, errs []error) {
	byName := map[string]Skill{}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(dir, "*", "SKILL.md"))
		sort.Strings(matches)
		for _, m := range matches {
			s, err := ParseSkill(filepath.Dir(m))
			if err != nil {
				errs = append(errs, err)
				continue
			}
			byName[s.Name] = s
		}
	}
	for _, s := range byName {
		skills = append(skills, s)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, errs
}

var (
	skillsMu sync.RWMutex
	skills   []Skill
)

// SetSkills records the session's skills; the skill tool is offered only
// when there is at least one.
func SetSkills(s []Skill) {
	skillsMu.Lock()
	defer skillsMu.Unlock()
	skills = append([]Skill(nil), s...)
}

// Skills returns the session's skills.
func Skills() []Skill {
	skillsMu.RLock()
	defer skillsMu.RUnlock()
	return append([]Skill(nil), skills...)
}

// skillSchemas is the skill tool, listing every skill, or nothing when the
// session has none.
func skillSchemas() []ToolSchema {
	list := Skills()
	if len(list) == 0 {
		return nil
	}
	names := make([]string, len(list))
	var desc strings.Builder
	for i, s := range list {
		names[i] = s.Name
		fmt.Fprintf(&desc, "\n- %s: %s", s.Name, s.Description)
	}
	enum, _ := json.Marshal(names)
	return []ToolSchema{{
		Type: "function",
		Function: FunctionDef{
			Name: "skill",
			Description: "Load a skill: instructions for a kind of task, written by the user. " +
				"When the task matches a skill's description, load it before starting and follow it. " +
				"A skill may mention supporting files in its folder; read one by passing its path as file.\n\nSkills:" + desc.String(),
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string", "enum": ` + string(enum) + `},
					"file": {"type": "string", "description": "A supporting file, relative to the skill's folder. Omit to load SKILL.md."}
				},
				"required": ["name"]
			}`),
		},
	}}
}

// LoadSkillTool runs the skill tool: SKILL.md's instructions, or one of the
// skill's supporting files (which must stay inside its folder).
func LoadSkillTool(name, file string) (string, error) {
	for _, s := range Skills() {
		if s.Name != name {
			continue
		}
		if file == "" {
			return fmt.Sprintf("Skill %q (folder: %s)\n\n%s", s.Name, s.Dir, s.Body), nil
		}
		path, err := confinePath(s.Dir, filepath.Join(s.Dir, file))
		if err != nil {
			return "", fmt.Errorf("file must be inside the skill's folder: %w", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return ListDir(path)
		}
		if info.Size() > maxSkillFile {
			return "", fmt.Errorf("%s is %d KB; skill files are limited to %d KB", file, info.Size()>>10, maxSkillFile>>10)
		}
		b, err := os.ReadFile(path)
		return string(b), err
	}
	return "", fmt.Errorf("unknown skill %q", name)
}
