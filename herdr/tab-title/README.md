# tab-title

Claude Code の応答が終わるたびに、会話内容から10文字以内のタイトルを作り、herdr のタブ名に設定する Stop フック。

## 仕組み

1. Stop フックで `herdr-tab-title.sh` が起動し、セッションの記録（transcript）からユーザー発言を取り出す
2. 最初の発言と直近5発言を `claude -p --model haiku` に渡し、タブ名を1つ作らせる
3. `herdr tab rename $HERDR_TAB_ID <タイトル>` で自分のタブだけを改名する

タイトル生成はバックグラウンドで動くため、応答の終了は待たされない。

## 更新のタイミング

- 初回の応答完了時
- その後はユーザー発言が `RETITLE_EVERY`（既定 8）回増えるごと

## 命名ルール（プロンプトに記載）

- GitHub の PR URL は `#197`、レビュー依頼なら英語で、空白なしの `#197review` とする（日本語の「レビュー」にはしない。空白を入れると10文字を超える）
- Issue URL は `#12`
- それ以外は作業の対象と目的が分かる名詞句

## 誤動作を防ぐ仕組み

- `HERDR_ENV` が無ければ何もしない（herdr の外では動かない）
- 子プロセスの `claude -p` には `HERDR_TAB_TITLER=1` を渡し、Stop フックが再帰的に動かないようにする
- 子プロセスでは `HERDR_ENV` を外し、herdr 管理の状態通知フックがこのペインの状態を誤って更新しないようにする

## セットアップ

1. スクリプトを `~/.claude/hooks/` からリンクする

   ```bash
   ln -s "$(pwd)/herdr-tab-title.sh" ~/.claude/hooks/herdr-tab-title.sh
   ```

2. `~/.claude/settings.json` の `hooks.Stop` に追加する

   ```json
   {
     "matcher": "*",
     "hooks": [
       { "type": "command", "command": "~/.claude/hooks/herdr-tab-title.sh", "timeout": 10 }
     ]
   }
   ```

3. 起動中の Claude セッションは入り直す（フック設定は起動時に読み込まれるため）

## 依存

`jq`、`python3`、`claude`、`herdr`（いずれも PATH 上にあること）
