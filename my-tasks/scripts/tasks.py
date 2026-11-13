#!/usr/bin/env python3
"""複数の GitHub Project から自分担当のアイテムを集め、ステータスを一覧・変更する。

使い方:
  tasks.py list [--all]            自分担当の未完了アイテムを一覧（--all で完了済みも含む）
  tasks.py statuses                各プロジェクトで選べるステータスを表示
  tasks.py set <番号> <ステータス>  直前の list の番号でステータスを変更
  tasks.py draft add <タイトル>     チケット化前の下書きをローカルに追加
  tasks.py draft rm <D番号>         下書きを削除（例: draft rm D2）
"""
import json
import os
import subprocess
import sys
from datetime import date
from pathlib import Path

# 対象プロジェクトは環境ごとに違うため、リポジトリの外の設定ファイルから読む
CONFIG = Path(os.environ.get("MY_TASKS_CONFIG",
                             Path.home() / ".config" / "my-tasks" / "config.json"))


def load_projects():
    if not CONFIG.exists():
        raise SystemExit(f"設定ファイルがありません: {CONFIG}\n"
                         "config.example.json をコピーして作成してください")
    return json.loads(CONFIG.read_text())["projects"]


PROJECTS = load_projects()
CACHE = Path.home() / ".cache" / "my-tasks" / "last.json"
# 下書きはキャッシュと違い消えては困るため、別の場所に保存する
DRAFTS = Path.home() / ".local" / "share" / "my-tasks" / "drafts.json"


def gh(*args):
    out = subprocess.run(["gh", *args], check=True, capture_output=True, text=True)
    return json.loads(out.stdout) if out.stdout.strip() else None


def status_field(p):
    fields = gh("project", "field-list", str(p["number"]), "--owner", p["owner"], "--format", "json")
    for f in fields["fields"]:
        if f["name"] == "Status":
            return f
    raise SystemExit(f"project {p['number']} に Status 項目がありません")


def cmd_list(include_done):
    rows = []
    for p in PROJECTS:
        query = "assignee:@me"
        if not include_done:
            query += "".join(f' -status:"{s}"' for s in p["done"])
        data = gh("project", "item-list", str(p["number"]), "--owner", p["owner"],
                  "--limit", "500", "--format", "json", "--query", query)
        project_id = gh("project", "view", str(p["number"]), "--owner", p["owner"],
                        "--format", "json")["id"]
        for it in data["items"]:
            if not include_done and "include" in p and it.get("status") not in p["include"]:
                continue
            c = it.get("content", {})
            repo = (c.get("repository") or "").split("/")[-1]
            rows.append({
                "project": p["key"], "project_number": p["number"], "owner": p["owner"], "project_id": project_id,
                "item_id": it["id"], "status": it.get("status") or "(未設定)",
                "type": "PR" if c.get("type") == "PullRequest" else c.get("type", "Draft"),
                "ref": f"{repo}#{c['number']}" if c.get("number") else "-",
                "title": it.get("title", ""), "url": c.get("url", ""),
            })
    rows.sort(key=lambda r: (r["project"], r["status"], r["ref"]))
    CACHE.parent.mkdir(parents=True, exist_ok=True)
    CACHE.write_text(json.dumps(rows, ensure_ascii=False, indent=1))
    print("| # | PJ | Status | Type | Ref | Title |")
    print("|---|----|--------|------|-----|-------|")
    for i, r in enumerate(rows, 1):
        title = r["title"].replace("|", "\\|")
        link = f"[{r['ref']}]({r['url']})" if r["url"] else r["ref"]
        print(f"| {i} | {r['project']} | {r['status']} | {r['type']} | {link} | {title} |")
    print(f"\n合計 {len(rows)} 件")
    print_drafts()


def load_drafts():
    return json.loads(DRAFTS.read_text()) if DRAFTS.exists() else []


def save_drafts(drafts):
    DRAFTS.parent.mkdir(parents=True, exist_ok=True)
    DRAFTS.write_text(json.dumps(drafts, ensure_ascii=False, indent=1))


def print_drafts():
    drafts = load_drafts()
    print("\n### 下書き（ローカル・未チケット化）\n")
    if not drafts:
        print("（なし）")
        return
    print("| # | 追加日 | Title |")
    print("|---|--------|-------|")
    for i, d in enumerate(drafts, 1):
        print(f"| D{i} | {d['created']} | {d['title'].replace('|', '\\|')} |")


def cmd_draft(args):
    drafts = load_drafts()
    if args and args[0] == "add" and len(args) >= 2:
        title = " ".join(args[1:])
        drafts.append({"title": title, "created": date.today().isoformat()})
        save_drafts(drafts)
        print(f"D{len(drafts)} を追加: {title}")
    elif args and args[0] == "rm" and len(args) == 2:
        index = int(args[1].lstrip("Dd"))
        if not 1 <= index <= len(drafts):
            raise SystemExit(f"番号は D1〜D{len(drafts)} で指定してください")
        removed = drafts.pop(index - 1)
        save_drafts(drafts)
        print(f"D{index} を削除: {removed['title']}")
    else:
        print_drafts()


def cmd_statuses():
    for p in PROJECTS:
        opts = [o["name"] for o in status_field(p)["options"]]
        print(f"{p['key']} (project {p['number']}): " + " / ".join(opts))


def cmd_set(index, status):
    if not CACHE.exists():
        raise SystemExit("先に list を実行してください")
    rows = json.loads(CACHE.read_text())
    if not 1 <= index <= len(rows):
        raise SystemExit(f"番号は 1〜{len(rows)} で指定してください")
    r = rows[index - 1]
    field = status_field({"number": r["project_number"], "owner": r["owner"]})
    match = [o for o in field["options"] if o["name"].lower() == status.lower()]
    if not match:
        names = " / ".join(o["name"] for o in field["options"])
        raise SystemExit(f"'{status}' は {r['project']} のステータスにありません: {names}")
    subprocess.run(["gh", "project", "item-edit", "--id", r["item_id"],
                    "--project-id", r["project_id"], "--field-id", field["id"],
                    "--single-select-option-id", match[0]["id"]],
                   check=True, capture_output=True)
    print(f"#{index} {r['ref']} {r['title']}: {r['status']} → {match[0]['name']}")
    r["status"] = match[0]["name"]
    CACHE.write_text(json.dumps(rows, ensure_ascii=False, indent=1))


if __name__ == "__main__":
    a = sys.argv[1:]
    if not a or a[0] == "list":
        cmd_list("--all" in a)
    elif a[0] == "statuses":
        cmd_statuses()
    elif a[0] == "draft":
        cmd_draft(a[1:])
    elif a[0] == "set" and len(a) >= 3:
        cmd_set(int(a[1]), " ".join(a[2:]))
    else:
        print(__doc__)
        sys.exit(1)
