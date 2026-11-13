#!/usr/bin/env bash
# 「<ワークスペース名>:<ペイン>」(例: myapp:p0) を Herdr のペインIDに変換する。
# 使い方: resolve-target.sh "myapp:p0"
# 成功時: 標準出力にJSON {"pane_id","workspace_id","agent","agent_status","cwd"} を1行で出す。
# 失敗時: 理由を標準エラーに出して終了コード1。
set -euo pipefail

raw="${1:-}"
# プロンプト記号や "in " などの前置きを取り除き、最後の「語:語」だけを使う
spec=$(printf '%s' "$raw" | grep -oE '[^[:space:]:]+:[^[:space:]:]+' | tail -n1 || true)
[ -n "$spec" ] || { echo "対象の形式が読み取れません: '$raw' (例: myapp:p0)" >&2; exit 1; }

ws_label="${spec%:*}"
pane_part="${spec##*:}"

ws_ids=$(herdr workspace list | jq -r --arg l "$ws_label" '.result.workspaces[] | select(.label==$l) | .workspace_id')
count=$(printf '%s\n' "$ws_ids" | grep -c . || true)
[ "$count" -ge 1 ] || { echo "ワークスペース '$ws_label' が見つかりません" >&2; exit 1; }
[ "$count" -eq 1 ] || { echo "ワークスペース名 '$ws_label' が複数あります: $(echo $ws_ids)" >&2; exit 1; }

# ペインIDの後半(p0)、またはペインのラベルのどちらでも一致させる
pane=$(herdr pane list --workspace "$ws_ids" | jq -c --arg ws "$ws_ids" --arg p "$pane_part" '
  [.result.panes[] | select(.pane_id == ($ws + ":" + $p) or .label == $p)]
  | if length == 1 then .[0] | {pane_id, workspace_id: $ws, agent, agent_status, cwd}
    else empty end')
[ -n "$pane" ] || { echo "ペイン '$pane_part' が '$ws_label' 内で一意に見つかりません" >&2; exit 1; }
printf '%s\n' "$pane"
