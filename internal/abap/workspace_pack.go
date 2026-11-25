package abap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

// WorkspaceInstructions are added to the pack instructions when the
// workspace is on.
const WorkspaceInstructions = `

Workspace (local files):
  abap_checkout  copy an object's SAP source into a local file and return its path
  abap_apply     deploy that file to SAP, refusing if SAP changed since checkout
- To CHANGE an existing object: abap_checkout it, edit the file (patch_file for small
  edits in large files; never rewrite a large file just to change a few lines), then
  abap_apply. Do not paste large sources inline.
- To CREATE an object: write its file at the path abap_checkout would use (the
  workspace naming, e.g. zcl_x.clas.abap), then abap_apply.
- If abap_apply reports a conflict, SAP changed since checkout: STOP and report it.
  Do not apply with force unless the user tells you to.`

// previewSource returns the first max lines and a pointer to abap_checkout.
func previewSource(src string, max int, a objectArgs) string {
	lines := strings.SplitAfter(src, "\n")
	total := CountLines(src)
	if len(lines) > max {
		lines = lines[:max]
	}
	return fmt.Sprintf("%s\n... (%d of %d lines shown) Use abap_checkout %s (type %s) to work on the full source as a file.",
		strings.TrimRight(strings.Join(lines, ""), "\n"), max, total, a.Name, a.Type)
}

const workspaceSchema = `{
	"type": "object",
	"properties": {
		"name":  {"type": "string", "description": "Object name, uppercase"},
		"type":  {"type": "string", "enum": ["program","class","interface","include","structure","table","ddls","srvd"]},
		"force": {"type": "boolean", "description": "Override the local-changes or conflict check. Only when the user asked for it."}
	},
	"required": ["name", "type"]
}`

type handlerFactory func(func(*Client, objectArgs) (string, error)) func(context.Context, string) (string, error)

func workspaceTools(env Env, withClient handlerFactory) []tools.PackTool {
	ws := env.Workspace
	return []tools.PackTool{
		{
			Name:        "abap_checkout",
			Description: "Copy an object's current SAP source into a local workspace file and remember that version, so a later abap_apply can detect changes made on SAP in between. Returns the file path and line count, not the source.",
			Parameters:  json.RawMessage(workspaceSchema),
			PrimaryArg:  "name",
			Handler: withClient(func(c *Client, a objectArgs) (string, error) {
				if err := a.requireWorkspaceObject(ws); err != nil {
					return "", err
				}
				path, lines, err := ws.Checkout(c, a.Name, a.Type, a.Force)
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("Checked out %s %s to %s (%d lines). Edit the file, then abap_apply.", a.Type, a.Name, displayPath(path), lines), nil
			}),
		},
		{
			Name:        "abap_apply",
			Description: "Deploy an object's workspace file to SAP (save + activate). For an object that was checked out, it first re-reads SAP and refuses if SAP changed since checkout. A new object must not exist on SAP yet.",
			Parameters:  json.RawMessage(workspaceSchema),
			PrimaryArg:  "name",
			Handler: withClient(func(c *Client, a objectArgs) (string, error) {
				if err := a.requireWorkspaceObject(ws); err != nil {
					return "", err
				}
				return applyFile(c, ws, env.Tracker, a.Name, a.Type, a.Force)
			}),
		},
	}
}

func (a objectArgs) requireWorkspaceObject(ws *Workspace) error {
	if err := a.requireObject(); err != nil {
		return err
	}
	if !ws.Supports(a.Type) {
		return fmt.Errorf("type %s cannot be worked on as a workspace file; use abap_deploy", a.Type)
	}
	return nil
}

// applyFile is abap_apply: conflict check, deploy from the file, new base.
func applyFile(c *Client, ws *Workspace, tr *Tracker, name, objectType string, force bool) (string, error) {
	path, err := ws.Path(name, objectType)
	if err != nil {
		return "", err
	}
	src, err := ws.Read(name, objectType)
	if err != nil {
		return "", err
	}
	if !force {
		if err := ws.CheckConflict(c, name, objectType); err != nil {
			return "", err
		}
	}
	err = deploy(c, name, objectType, src)
	if tr != nil {
		tr.RecordFile(name, objectType, path, err)
	}
	if err != nil {
		return "", err
	}
	if err := ws.Applied(name, objectType, src); err != nil {
		return "", fmt.Errorf("deployed, but recording the new base failed: %w", err)
	}
	return fmt.Sprintf("Applied %s to SAP and activated %s %s (%d lines).", displayPath(path), objectType, name, CountLines(src)), nil
}

// IsConflict reports whether err is a workspace conflict.
func IsConflict(err error) bool { return errors.Is(err, ErrConflict) }

// displayPath shows path relative to the working directory when it is inside it.
func displayPath(path string) string {
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}
