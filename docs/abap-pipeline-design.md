# ABAP pipeline: local files, lint, git and pluggable deploy (bai CLI first)

Status: proposal. Scope: `bai abap` / `abaper` in this repository. The web
app (cai), iOS (cai-ios) and Android (cai-android) clients and any
server-side host come later and reuse the same pipeline package.

## Goals

- Work on ABAP objects as **files on disk**, not as strings held in memory or
  in the model's context, so objects of thousands of lines stay cheap.
- **Lint** (abaplint), **git** and **GitHub** as optional, pluggable stages.
- **Deploy** through a pluggable deployer: ADT (default), abapGit, or manual.
- **Everything is opt-in and off by default.** With no `abap:` settings,
  `bai abap` behaves exactly as today.
- The CLI can also hand the pipeline to a **remote runner** (a server, or
  `bai` on a VM) instead of running it locally.

## Non-goals (for now)

- Line-level patching on SAP. Neither ADT nor abapGit offers it: the agent
  edits files locally and the whole object is applied.
- abapGit inside the agent's edit/repair loop. abapGit is a batch
  deploy/promote step (see Deployers).
- Server-side hosting. It is a later decision (abaper serve, `bai` on a user
  VM, or a dedicated service); this design only fixes the contract.

## One pipeline, two entry points

```
[checkout] → edit files → [lint] → [commit] → deploy → verify → [push]
```

Stages in brackets are plugins and are skipped when not configured. Deploy is
always present; its default plugin is ADT, which is what `bai abap` does today.

- **New artifacts:** no checkout. The agent writes files, then deploys.
- **Existing code:** `checkout` reads the object from SAP into the workspace
  and records a **base hash** of what SAP held. The agent edits the file
  (with `patch_file` for large objects), then **apply** re-reads SAP and
  refuses to overwrite if SAP changed since checkout (conflict), before the
  whole object is deployed.

## Workspace

- A directory (default `abap-src/` at the project root) holding one file per
  object, using abapGit file names where abapGit has a source file:

  | type | file |
  |---|---|
  | class | `zcl_x.clas.abap` |
  | interface | `zif_x.intf.abap` |
  | program, include | `zx.prog.abap` |
  | ddls | `zi_x.ddls.asddls` |
  | srvd | `zsd_x.srvd.srvdsrv` |
  | table, structure | `zt_x.tabl.astabl` (bai naming; abapGit stores TABL as XML) |

- `.abaper-state.json` in the workspace records, per object, the file and the
  SHA-256 of the SAP source at checkout or last apply (the base).
- The deploy tracker stores the file path and hash, not the source. The
  verify gate reads the source from the file.
- Tool results stay small: reading a large object from SAP writes it to the
  workspace and returns the path and line count, not the text.

Known gap: the ADT client reads only a class's main source. Local
definitions, implementations, macros and test classes
(`.clas.locals_imp.abap`, `.clas.testclasses.abap`, ...) need client
support before checkout of such classes is safe; until then checkout refuses
classes that have them, or the gap is reported clearly.

## Plugins

Compile-time Go interfaces, chosen by name in settings. No dynamic Go plugins
(not portable). External tools run as subprocesses; a missing tool means the
stage is skipped with a notice, never a failed run.

| stage | interface | plugins |
|---|---|---|
| lint | `Linter` | `abaplint` (Node CLI, MIT) |
| commit | `VCS` | `git` |
| push | `Remote` | `github` (git push, optional PR via `gh` or GitHub MCP) |
| deploy | `Deployer` | `adt` (default), `abapgit`, `manual` |
| host | `Runner` | `local` (default), `remote` |

### Lint

Reuses the abaplint runner that the language server already has
(`abaper/internal/lsp/abaplint`). It moves to a package `internal/abap` can
import. Findings at `fail_on` severity (default `error`) fail the object and go
back to the agent through the existing repair rounds. CDS and service objects
are skipped (abaplint targets ABAP source).

### Git and GitHub

- Commit before deploy, so a failed activation still leaves the work on the
  branch. Branch per run by default; never `main`.
- With the ADT deployer, push only after verify passes.
- With the abapGit deployer, push to a working branch first (abapGit pulls
  from the remote), then verify, then merge or promote.

### Deployers

- `adt`: lock, PUT the full source, unlock, activate. Per-object feedback for
  the repair loop.
- `abapgit`: optional and probed. abapGit may be missing on the target
  system, so the plugin checks for it (and for the API it calls) and asks the
  user when unsure. Candidate APIs: the BlueRequests gateway's existing
  `/rest/git/sap/v1/repo` service (list/create repos today), or
  `abapgit-api-rfc` (`ZABAPGIT_API_RFC_PULL`). The SAP system must reach the
  git remote.
- `manual`: prepare files and the branch, print what to pull or import.

### Remote runner (optional)

`runner: remote` sends the stages to a pipeline service instead of running
them on this machine (for users without git, Node or local disk, or for a
shared VM). The local runner is the reference implementation; the remote one
speaks the same contract. SAP credentials keep travelling per request; the
remote side never stores them.

## Settings

Project `.bai/settings.yaml` overrides user `~/.bai/settings.yaml`. Absent =
off.

```yaml
abap:
  workspace:
    enabled: false
    dir: abap-src
    inline_max_lines: 200   # larger sources go to files, not tool output
  pipeline:
    runner: local           # local | remote
    lint:   { plugin: abaplint, fail_on: error }
    vcs:    { plugin: git, branch: "bai/{run}", commit: per-run }
    remote: { plugin: github, push: after-verify }
    deploy: { plugin: adt }  # adt | abapgit | manual
```

## Phases

1. Workspace: settings, file-backed tracker, `abap_checkout` / `abap_apply`
   with the base-hash conflict check, large reads to files. No plugins.
2. Lint plugin (move the abaplint runner), verify-gate lint step.
3. Git and GitHub plugins.
4. Deployer interface; abapGit and manual deployers.
5. Remote runner.

Each phase is off by default and leaves existing behaviour unchanged.
