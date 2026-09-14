# agent-skills

Claude Code and Codex 向けの reusable skills / slash commands を管理するリポジトリ。

旧称は `claude-code-skills`。Claude Code 専用ではなくなったため、リポジトリ名は `agent-skills` を推奨する。

## Runtime Notes

- `commands/*.md` は Claude Code の custom slash commands として使う。
- `agents/openai.yaml` は Codex の skill UI metadata として使う。
- `allowed-tools` などの frontmatter は Claude Code 向け metadata として残す。
- 特定 runtime 専用の skill は README と skill description で明記する。

## Skills And Commands

### /prompt-token-reviewer

- `~/.claude/history.jsonl` と `~/.codex/sessions/` に保存されている過去の会話履歴から、ユーザーのプロンプトをランダムに10件抽出する
- 抽出したプロンプトに対してよりトークンを消費しない効率的な指示の仕方についてレビューを行う。

### /coupling-balance-review

- ソフトウェア設計、アーキテクチャ、モジュール境界、サービス境界、API、DB所有境界、イベント契約、クラス/パッケージ設計を結合のバランス観点でレビューする。
- モジュール結合、コナーセンス、統合強度、距離、変動性、均衡結合の観点を使い、単純な疎結合化ではなく変更コストに対して結合が均衡しているかを評価する。

### /research-design-review

- 調査開始前に、意思決定、調査目的、仮説、前提、証拠ギャップ、調査範囲、検証方法、成功基準を一問ずつ詰める。
- プロダクト調査、技術調査、セキュリティ調査、設計改善、障害原因調査、移行計画などで、いきなり結論を出す前の調査設計レビューとして使う。

### /herdr-treehouse-pane

- Codex / Herdr / Treehouse 向け。Herdr の別 pane に作業を依頼するとき、Treehouse-managed Git worktree 上で agent を起動する。
- 作業完了後の review / commit / push / cleanup は通常の Herdr / agent 運用に委ねる。

### /codex-rewind

- Codex のローカルセッションを指定 turn より前または指定 turn までの状態に clean clone する。
- 元セッションは削除せず、新しい session id と別 terminal で実行する `codex resume <new_session_id>` を表示する。
