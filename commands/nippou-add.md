---
name: nippou-add
description: Obsidian daily note の ### Nippo 欄に引数を箇条書きで追記する
---

# Nippo 追記コマンド

## 引数

```text
/nippou-add <追記内容>
```

引数として渡された内容: `$ARGUMENTS`

## エージェントへの指示

`$ARGUMENTS` をそのまま 1 件の Nippo として扱い、以下を実行する。

```bash
if [ -x "$HOME/.codex/skills/nippou-add/bin/nippou-add" ]; then
  "$HOME/.codex/skills/nippou-add/bin/nippou-add" "$ARGUMENTS"
else
  "$HOME/.claude/skills/nippou-add/bin/nippou-add" "$ARGUMENTS"
fi
```

実行後、更新した daily note のパスと追記した箇条書きを日本語で簡潔に返す。

## 注意

- `$ARGUMENTS` を要約・翻訳・整形しない。
- daily note が存在しない場合はスクリプトが Obsidian の Daily Notes 設定に従ってテンプレートから作成する。
- `### Nippo` が存在しない場合はスクリプトが末尾に追加する。
