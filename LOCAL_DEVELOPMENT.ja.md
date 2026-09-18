# ローカルDocker開発環境

Docker Desktopを起動して、リポジトリのルートで実行します。

```powershell
powershell -ExecutionPolicy Bypass -File scripts/start-dev.ps1
```

初回のみランダムな秘密情報を含む`.env`を作成します。既存の`.env`は上書きしません。別環境の設定が残っている場合は、ローカルストレージとHTTPの開発用オリジンを確認してください。`.env`と`data/`はGit管理対象外です。

- アプリ: http://127.0.0.1:8080
- 登録確認メール（Mailpit）: http://127.0.0.1:8025
- PostgreSQL: 127.0.0.1:5432、DB/ユーザー `glipz`、開発用パスワード `glipz_dev`
- Redis: 127.0.0.1:6379

登録画面からアカウントを作り、Mailpitで確認リンクを開きます。メールは外部送信されません。Push配信とPatreon連携は未設定です。全公開ポートをループバックに限定しています。この固定DB資格情報を含むComposeはローカル専用です。

```powershell
# 変更後の再ビルドと起動
docker compose up -d --build --wait
# 稼働状態・ログ
docker compose ps
docker compose logs --tail 100 backend
# 停止（データは保持）
docker compose down
# 実APIのセキュリティ回帰テスト（Python 3）
python scripts/security-smoke.py
```

ソース変更の反映には再ビルドが必要です。DBは`pgdata`ボリューム、メディアは`data/media`に保存します。`docker compose down -v`はDBを消すため、通常の停止には使わないでください。

回帰スクリプトはローカルのアプリとMailpitだけに接続し、一時ユーザーを作成して検査後に削除します。

## UI回帰テスト

Docker起動後、`web`で実行します。

```powershell
npm ci
npm test
npm run typecheck
npx playwright install chromium
npm run test:e2e
```

ブラウザーテストはローカルのMailpitでメール確認済みの一時アカウントを作成し、終了時に削除します。Python 3が必要です（実行ファイルは`PYTHON`環境変数で指定可能）。既存Chromiumを使う場合は`PLAYWRIGHT_CHROMIUM_EXECUTABLE`を指定します。途中で強制終了した場合は、リポジトリのルートで`python scripts/ui-fixture.py cleanup`を実行してください。テスト対象は320・390・768・1024・1440px、認証フォーム、投稿、テーマ、キーボードでのダイアログ操作、取得失敗からの復帰です。
