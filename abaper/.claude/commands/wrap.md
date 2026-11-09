---
description: Open a PR that closes the issue with a release-note-ready summary
argument-hint: [issue-number]
allowed-tools: Bash(git:*), Bash(gh pr:*), Read
---
Push the current branch and open a PR for issue #$ARGUMENTS:
- Title: Conventional Commit style.
- Body: "Closes #$ARGUMENTS", then a one-paragraph plain-English summary suitable for release notes, then the verify results.
