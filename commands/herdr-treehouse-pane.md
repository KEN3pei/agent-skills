---
name: herdr-treehouse-pane
description: Herdr の別 pane に Treehouse worktree 上で agent 作業を依頼する
---

# Herdr Treehouse Pane Command

Use `$herdr-treehouse-pane` to delegate the following request to another Herdr pane.

The delegated pane must start `treehouse` first and then start the requested coding agent inside the Treehouse subshell. Default to a sibling pane in the current tab. Use a new tab only if the user explicitly asks for tab-based delegation.

Do not use `treehouse get --lease` unless the user explicitly asks for a durable lease. Do not manage review, commit, push, agent shutdown, or Treehouse cleanup as part of this command.

Delegated request:

```text
$ARGUMENTS
```
