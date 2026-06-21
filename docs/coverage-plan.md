Summary
-------
Current test coverage (after latest run): 4.3% overall.
Module highlights:
- internal/grpc: 49.6%
- internal/ui: 30.7%
- internal/config: 5.2%
- internal/cmd: 3.2%
- many packages (api/proto, cmd/bai, internal/auth, internal/audit, internal/hooks, internal/mcp, internal/session, internal/tools, internal/ui/tui) are at 0.0% (generated files like api/proto/bff are excluded from unit testing targets).

Goal
----
Raise overall coverage to >= 50% across the repository in incremental milestones, starting with high-impact packages used by the CLI and connection layers.

Scope & Priorities
------------------
1. High priority (quick wins, high impact):
   - internal/cmd: add unit tests for commands using bufconn testBFF harness (existing cmd_test.go) to exercise RPC call paths, flags parsing, and output modes (table/json/quiet).
   - internal/config: tests for Load/Save, default backfills, and token validation logic.
   - internal/ui: tests for output formats, encoders, quiet mode, and stream renderer edge cases.
2. Medium priority:
   - internal/grpc: expand transport/unit tests for interceptors and TLS auto-detection (already at ~50%).
   - internal/auth: device flow happy path and error handling.
3. Lower priority:
   - internal/mcp, internal/hooks, internal/session, internal/tools: add unit tests for public functions where feasible.
   - Generated packages (api/proto/*) should be excluded from coverage targeting or ignored in coverage report.

Concrete Tasks
--------------
- Add new tests to internal/cmd:
  - Exercises: chat commands (list/start/history/context/title/stop), model commands, login flow stubbed via testBFF responses.
  - Use the existing bufconn harness; add canned responses to testBFF in cmd_test.go and validate stdout/stderr via captured buffers.
- Add tests to internal/config:
  - Roundtrip YAML load/save, default field backfills, invalid token cases.
- Add tests to internal/ui:
  - Table & JSON encoding correctness, quiet mode behavior, stream renderer rendering with short/long inputs and cancellation.
- Add tests to internal/grpc for auth interceptor behavior using mock credentials and bufconn.

Milestones & Timeline (rough estimate)
-------------------------------------
- Week 1: Add 20–30 focused tests in internal/cmd and internal/config — expected increase to ~25–30%.
- Week 2: Add UI + more grpc tests — expected increase to ~45–55%.
- Week 3: Backfill remaining packages and CI checks — reach >= 50% and add coverage gating.

CI & Repo Changes
------------------
- Add a make target (already exists: test-cover) and consider enabling a CI job that fails when coverage < 50%.
- Exclude generated proto code from coverage (add a coverage profile filter or move generated files to a path excluded by CI step).

Deliverables
------------
- Branch: feat/coverage-plan (this branch contains this plan file).
- Incremental PRs grouped by package (cmd, config, ui, grpc) with tests and small implementation fixes as needed.
- Final PR to add CI coverage gating once 50% is achieved.

Risk & Notes
-----------
- Some packages depend on external systems; tests should use bufconn/mocks to avoid network.
- Keep tests fast and deterministic (use -race in CI as current Makefile requires).
- Avoid adding heavy external dependencies; prefer built-in testing helpers and test harnesses already present in repo.
