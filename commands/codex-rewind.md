---
name: codex-rewind
description: Codex のローカルセッションを指定 turn より前の状態に clean clone し、別 terminal で実行する resume コマンドを表示する
---

# Codex Rewind コマンド

## 引数

```text
/codex-rewind <session_id> [--before-turn <turn_id> | --through-turn <turn_id>]
```

引数として渡された内容: `$ARGUMENTS`

## Claude への指示

`/Users/uenokensuke/Apps/cc-skills/codex-rewind/SKILL.md` を読み、以下のGo moduleを使う。

```bash
cd /Users/uenokensuke/Apps/cc-skills/codex-rewind/scripts/codex-rewind
go run . list-turns --session-id <session_id>
go run . clone-before-turn --session-id <session_id> --before-turn <turn_id>
go run . clone-through-turn --session-id <session_id> --through-turn <turn_id>
```

境界turnが不明な場合は `list-turns` の結果から、最初に除外したいturnか最後に残したいturnをユーザーに確認する。

完了後は、生成された新しい session id と、別terminalで実行する以下の形式のコマンドだけを明確に返す。

```bash
codex resume <new_session_id>
```

## 注意

- 元セッションは削除しない。
- 生成された `codex resume` コマンドを自分では実行しない。
- 現在動作中のCodex paneの文脈は変わらない。
