#!/usr/bin/env python3
"""Herdr のワークスペース・タブ・ペインをツリー表示する（読み取りのみ）。

使い方: tree.py [ワークスペース名]   名前を指定するとそのワークスペースだけ表示する
"""
import json
import os
import subprocess
import sys

HOME = os.path.expanduser("~")


def herdr(*args):
    out = subprocess.run(["herdr", *args], capture_output=True, text=True, check=True)
    return json.loads(out.stdout)["result"]


def main():
    only = sys.argv[1] if len(sys.argv) > 1 else None
    me = os.environ.get("HERDR_PANE_ID")
    workspaces = [w for w in herdr("workspace", "list")["workspaces"]
                  if only is None or w["label"] == only]
    if not workspaces:
        raise SystemExit(f"ワークスペース '{only}' が見つかりません")
    for w in workspaces:
        print(f"{w['label']} ({w['workspace_id']}) [{w['agent_status']}]")
        tabs = herdr("tab", "list", "--workspace", w["workspace_id"])["tabs"]
        panes = herdr("pane", "list", "--workspace", w["workspace_id"])["panes"]
        for ti, t in enumerate(tabs):
            last_tab = ti == len(tabs) - 1
            print(f"{'└─' if last_tab else '├─'} {t['label']} ({t['tab_id']}) [{t['agent_status']}]")
            tab_panes = [p for p in panes if p["tab_id"] == t["tab_id"]]
            for pi, p in enumerate(tab_panes):
                branch = "└─" if pi == len(tab_panes) - 1 else "├─"
                # session push/pull でそのまま使える「ワークスペース名:ペイン」形式で出す
                spec = f"{w['label']}:{p['pane_id'].split(':', 1)[1]}"
                kind = p.get("agent") or "shell"
                detail = p.get("terminal_title_stripped") or p["foreground_cwd"].replace(HOME, "~")
                mine = " ◀ このセッション" if p["pane_id"] == me else ""
                print(f"{'   ' if last_tab else '│  '}{branch} {spec} {kind} [{p['agent_status']}] {detail[:50]}{mine}")


if __name__ == "__main__":
    main()
