---
name: prompt-token-reviewer
description: Review prompt token efficiency from local Claude Code and Codex prompt history. Use when the user invokes "/prompt-token-reviewer", asks to review prompt history, or wants to improve prompting efficiency.
allowed-tools: Bash
disable-model-invocation: true
---

Claude Code と Codex のローカル会話履歴を対象に、以下の手順でプロンプトのトークン効率レビューを実施してください。

## 手順

1. 次のコマンドを実行して、`~/.claude/history.jsonl` と `~/.codex/sessions/` からランダムに抽出したユーザープロンプト一覧を取得する：

```bash
if [ -d "$HOME/.codex/skills/prompt-token-reviewer/script" ]; then
  cd "$HOME/.codex/skills/prompt-token-reviewer/script"
else
  cd "$HOME/.claude/skills/prompt-token-reviewer/script"
fi
go run .
```

2. 出力された番号付きプロンプト（`N. <プロンプト全文>` 形式）を受け取り、各プロンプトのトークン効率を日本語でレビューする。

## 出力形式

10件まとめて、以下の番号付きリスト形式で出力すること：

---

**[N] 元のプロンプト:**
（100文字を超える場合は先頭100文字程度に省略して表示）

**問題点:** トークン効率の悪い箇所（冗長な表現・不要な修飾語・重複表現など）を具体的に指摘

**改善案:** より短く・明確な表現の提案

---
