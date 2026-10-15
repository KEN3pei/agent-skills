#!/bin/bash
# Claude Code の Stop フック: 会話内容から10文字以内のタイトルを作り、herdr のタブ名に設定する。
# - 初回の応答完了時と、その後 RETITLE_EVERY 回のユーザー発言ごとに更新する
# - タイトル生成は claude -p (haiku) をバックグラウンドで実行し、フック自体はすぐ終了する
set -u

RETITLE_EVERY=8
MAX_CHARS=10

# herdr の外、または タイトル生成用の子プロセスから呼ばれた場合は何もしない
[ "${HERDR_ENV:-}" = "1" ] || exit 0
[ -z "${HERDR_TAB_TITLER:-}" ] || exit 0
[ -n "${HERDR_TAB_ID:-}" ] || exit 0
command -v jq >/dev/null 2>&1 || exit 0
command -v herdr >/dev/null 2>&1 || exit 0

input="$(cat)"
transcript="$(printf '%s' "$input" | jq -r '.transcript_path // empty')"
session_id="$(printf '%s' "$input" | jq -r '.session_id // empty')"
[ -f "$transcript" ] && [ -n "$session_id" ] || exit 0

# ユーザー発言（ツール結果を除く）のテキストを抜き出す
user_texts() {
  jq -r 'select(.type == "user" and (.isSidechain // false) == false)
         | .message.content
         | if type == "string" then .
           else map(select(.type == "text") | .text) | join("\n") end
         | select(length > 0)' "$transcript" 2>/dev/null
}

count="$(user_texts | grep -c . || true)"
[ "${count:-0}" -ge 1 ] || exit 0

state_dir="${TMPDIR:-/tmp}/herdr-tab-title"
mkdir -p "$state_dir"
state_file="$state_dir/$session_id"
last_count=-1
[ -f "$state_file" ] && last_count="$(cat "$state_file")"

# 初回(-1)か、前回から RETITLE_EVERY 回以上ユーザー発言が増えたときだけ実行
if [ "$last_count" -ne -1 ] && [ $((count - last_count)) -lt "$RETITLE_EVERY" ]; then
  exit 0
fi
echo "$count" >"$state_file"

(
  # 最初の発言（話題の元）と直近の発言を渡す。長すぎないよう切り詰める
  context="$( { user_texts | head -n 1; user_texts | tail -n 5; } | cut -c1-300)"
  system="あなたはタブ名を作る専用の機械です。出力はタブ名1行のみ。見出し記号(#を文頭に置く Markdown 見出し)・引用符・説明・改行は禁止です。"
  prompt="次はコーディングセッションでのユーザー発言です。内容を表すタブ名を${MAX_CHARS}文字以内で1つだけ出力してください。

規則:
- GitHub の PR URL (…/pull/197) があれば「#197」を基本にし、レビュー依頼なら必ず英語で、空白なしの「#197review」とする（「レビュー」と日本語にしない。空白を入れると10文字を超える）
- Issue URL (…/issues/12) があれば「#12」を基本にする
- それ以外は、作業の対象と目的が分かる名詞句にする（例: タブ名自動化, herdr設定, BQコスト調査）
- 「結論」「質問」のような中身のない一般語は使わない

発言:
${context}"

  title="$(env -u HERDR_ENV HERDR_TAB_TITLER=1 claude -p --model haiku \
             --no-session-persistence --tools "" --system-prompt "$system" "$prompt" 2>/dev/null \
           | head -n 1 | tr -d '「」"'"'"'`\r' | sed 's/^#\{1,\}[[:space:]]\{1,\}//;s/^[[:space:]]*//;s/[[:space:]]*$//')"
  [ -n "$title" ] || exit 0

  # 文字数で切り詰める（バイトではなく文字単位）
  title="$(printf '%s' "$title" | python3 -c "import sys; print(sys.stdin.read()[:${MAX_CHARS}])")"
  herdr tab rename "$HERDR_TAB_ID" "$title" >/dev/null 2>&1
) >/dev/null 2>&1 &

exit 0
