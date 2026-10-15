---
name: herdr-treehouse-pane
description: Use when delegating work to another Herdr pane and the delegated agent should start inside a Treehouse-managed Git worktree. Default to pane; use a tab only when the user explicitly asks for tab-based delegation.
---

# Herdr Treehouse Pane

Use this skill when a user asks you to delegate work to another Herdr pane and the delegated agent should run in an isolated Treehouse-managed Git worktree.

This skill only prepares the delegated Herdr pane or explicitly requested tab and starts the agent inside the Treehouse subshell. It does not manage review, commit, push, agent shutdown, or Treehouse cleanup after the delegated work.

## Requirements

- Follow the existing Herdr skill and the installed `herdr` CLI syntax for all Herdr operations.
- Verify that the current process is running inside Herdr before controlling panes:

```bash
test "${HERDR_ENV:-}" = 1
```

- Use Treehouse's normal subshell mode by default:

```bash
treehouse
```

Do not use `treehouse get --lease` for this workflow unless the user explicitly asks for a durable lease.

## Delegation Workflow

Default to a sibling pane in the current tab. Preserve the caller's current working directory when creating the pane so `treehouse` runs from the intended repository:

```bash
herdr pane split --current --direction right --cwd "$PWD" --no-focus
```

Pick the split direction according to the existing Herdr skill's layout guidance. If the user explicitly asks for a new tab instead of a pane, create a tab with the same working directory and use the returned root pane.

In the delegated pane, start Treehouse first:

```bash
herdr pane run <pane-id> "treehouse"
```

Wait until the pane output shows that Treehouse entered a worktree, then start the requested agent in that same pane with `herdr pane run`:

```bash
herdr pane run <pane-id> "codex"
```

Use the agent command requested by the user. If no command is specified, use the local default implied by the surrounding request and available Herdr agent integrations.

After Herdr detects the agent as ready, send the delegated task with the installed `herdr agent` input command, such as `herdr agent send <pane-id> <text>` when that is the supported command in the local CLI.

## Boundaries

- Do not return the Treehouse worktree as part of this skill's setup workflow.
- Do not send `exit` to the delegated pane just to trigger Treehouse cleanup unless the user explicitly asks for shutdown or the surrounding task has already reached its normal cleanup phase.
- Do not assume local branches in the Treehouse worktree are durable state. Treat remote branches, commits, pull requests, or copied summaries as the durable handoff mechanisms when persistence is needed.
- If Treehouse fails to enter a worktree, inspect the pane output and report the blocker instead of starting the agent in the original repository checkout.
