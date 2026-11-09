---
description: Implement a GitHub issue end to end
argument-hint: [issue-number]
---
Issue:
!`gh issue view $ARGUMENTS 2>&1`

Implement this issue against its acceptance criteria.
- Stay within "In scope"; respect "Out of scope".
- Small commits, Conventional Commits, referencing (#$ARGUMENTS).
- Add/update tests.
- When done, run the "Verify with" commands and report results.
Stop and ask if you hit a decision the issue doesn't cover.
