---
description: Build, test, and self-review the current change
allowed-tools: Bash, Read, Grep, Glob
---
Current change:
!`git diff main...HEAD --stat`

1. Run the project's build and tests (see CLAUDE.md / Makefile).
2. Check the diff against the linked issue's acceptance criteria — mark each met / not met.
3. Flag anything risky, out-of-scope, or untested.
Give a concise PASS/FAIL summary.
