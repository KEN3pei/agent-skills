---
name: codex-rewind
description: Use when the user wants to create a new local Codex session that stops before a chosen turn, so later turns are absent from saved history and UI. Only prepare the rewound session and print the resume command.
---

# Codex Rewind

Create a clean clone of a local Codex session that ends at an earlier turn. This is for Codex CLI history stored under `$CODEX_HOME` or `~/.codex`.

This workflow creates a new session. It does not modify the current in-memory context, start a new pane, resume the clone, or delete the original session.

## Workflow

1. Identify the source Codex session id.
   - In Herdr, `herdr agent list` can show `agent_session.value`.
   - Otherwise inspect `~/.codex/session_index.jsonl` or use Codex's session picker.
2. List turns when the boundary is unclear:

```bash
cd codex-rewind/scripts/codex-rewind
go run . list-turns --session-id <session_id>
```

3. Create the clone:

```bash
cd codex-rewind/scripts/codex-rewind
go run . clone-before-turn --session-id <session_id> --before-turn <turn_id>
```

or:

```bash
cd codex-rewind/scripts/codex-rewind
go run . clone-through-turn --session-id <session_id> --through-turn <turn_id>
```

4. Return the generated `codex resume <new_session_id>` command for the user to run in another terminal.

## Safety

- Do not delete or truncate the source session.
- Do not run the generated `codex resume` command yourself unless the user explicitly asks.
- The helper backs up the source rollout, `session_index.jsonl`, `state_5.sqlite`, and `thread_history_1.sqlite` under `~/.codex/session-cleanup-backups/`.
- If the requested boundary is ambiguous, list recent turns and ask which turn should be the first removed turn or the last kept turn.
- This is non-official local history manipulation. If Codex storage schema changes, inspect before writing.
