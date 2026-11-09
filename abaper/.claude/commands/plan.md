---
description: Read a GitHub issue and produce a plan (no edits)
argument-hint: [issue-number]
allowed-tools: Bash(gh issue view:*), Read, Grep, Glob
---
Issue:
!`gh issue view $ARGUMENTS 2>&1`

Read the issue above and this repo's CLAUDE.md, then:
1. Restate the outcome and acceptance criteria in your own words.
2. List any ambiguities or decisions you need from me.
3. Propose a step-by-step plan: files to change, order, tests to add.
Do NOT edit any files. Wait for my approval.
