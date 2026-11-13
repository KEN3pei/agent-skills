---
name: herdr-tree
description: "Herdr 管理下のワークスペース・タブ・ペインをツリー表示する（読み取りのみ）。`/herdr-tree [ワークスペース名]`。「セッション一覧をツリーで」「herdr のペインを一覧して」「どのセッションが止まっている？」などで起動する。Requires HERDR_ENV=1."
---

# herdr-tree: ワークスペース・タブ・ペインのツリー表示

> Herdr 専用のスキルです。`<このスキルのディレクトリ>` は、この `SKILL.md` が置かれているディレクトリを指します。

## 1. Herdr の中で動いているか確認する

```bash
test "${HERDR_ENV:-}" = 1
```

失敗したら「Herdr の外で動いているため使えません」と伝えて止まります。

## 2. ツリーを表示する

```bash
python3 <このスキルのディレクトリ>/scripts/tree.py            # 全ワークスペース
python3 <このスキルのディレクトリ>/scripts/tree.py myapp      # 引数のワークスペースだけ
```

出力をコードブロックでそのまま表示します。読み取りのみで、Herdr の状態は変えません。

ペインは `myapp:pA` の形で出るので、そのまま `/herdr-session-push` や `/herdr-session-pull` の宛先に使えます。

表示の後に、次の状態のペインがあれば1行ずつ添えます。

- `blocked`: 承認や質問への回答待ちで止まっている
- `done`: 終わったが、まだ画面で確認されていない
