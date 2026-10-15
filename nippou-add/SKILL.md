---
name: nippou-add
description: Append a user-provided text argument as a Markdown bullet to the `### Nippo` section of today's Obsidian daily note. Use when the user invokes `@nippou-add ...`, asks to add an item to Nippo/nippou/daily report, or wants a short daily work log entry recorded in the Obsidian daily note. Creates today's daily note from the configured Obsidian daily note template if it does not exist.
---

# Nippou Add

## Overview

Append the exact user-provided content to today's Obsidian daily note under the `### Nippo` heading as a Markdown bullet.

Default vault:

```text
/Users/uenokensuke/Documents/Obsidian Vault
```

## Workflow

1. Treat the text after `@nippou-add` as the entry content. Preserve the user's wording.
2. Run the bundled script from the active skill installation:

   ```bash
   if [ -x "$HOME/.codex/skills/nippou-add/bin/nippou-add" ]; then
     "$HOME/.codex/skills/nippou-add/bin/nippou-add" '<entry text>'
   else
     "$HOME/.claude/skills/nippou-add/bin/nippou-add" '<entry text>'
   fi
   ```

3. Report the updated daily note path and the bullet that was appended.

## Behavior

- Read Obsidian's Daily Notes settings from `.obsidian/daily-notes.json`.
- Resolve today's daily note path from `folder` and `format`.
- If the daily note is missing, create it from the configured `template`.
- Replace common template tokens such as `{{date}}`, `{{time}}`, and `{{title}}` when creating the note.
- Append `- <entry text>` under `### Nippo`.
- If `### Nippo` is missing, add it at the end of the file and then append the bullet.
- Do not summarize, rewrite, translate, or reformat the entry text beyond adding the Markdown bullet prefix.

## Binary Options

Use options only when needed for testing or non-default vaults:

```bash
"$HOME/.codex/skills/nippou-add/bin/nippou-add" --date 2026-09-03 --vault '/path/to/vault' 'entry text'
```

The binary prints the updated file path and appended bullet.

If the binary is missing after moving this skill to another machine, rebuild it from source:

```bash
go build -o "$HOME/.codex/skills/nippou-add/bin/nippou-add" "$HOME/.codex/skills/nippou-add/scripts/nippou_add.go"
```
