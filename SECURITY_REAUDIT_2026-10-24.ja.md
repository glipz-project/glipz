# セキュリティ診断・修正（2026-10-24）

## 対象と結果

開始時点のmainは`c8997c8f`。依存パッケージの既知脆弱性、アカウント移行の外向きHTTPと暗号データ処理を点検し、既存の認証・連合・メディア保護テストと本番ビルドを実行した。本番環境への診断・デプロイは実施していない。

| 対象 | 修正前 | 修正後 |
| --- | --- | --- |
| npm依存（開発依存を含む） | 21件：Critical 3、High 4、Moderate 14 | `npm audit`で0件 |
| Go依存・標準ライブラリ | `govulncheck`で到達可能な脆弱性11件 | 到達可能0件、インポートしたパッケージの指摘0件 |
| 移行HTTPのSSRF（中） | URL検証後の実接続に標準Transportを使用し、DNS再解決時のIPを未検証。localhostも無条件に許可 | 公開IPだけに接続する共通DialContextを使用。環境プロキシを無効化し、loopbackは明示設定時のみ許可 |
| 移行暗号データの不正形式（中） | 利用者が渡すnonce長を検証せずAES-GCMに渡し、リクエスト処理でpanicが発生する経路 | saltを16バイト、nonceを12バイトに制限し、KDF実行前に不正入力をエラーとして拒否 |
| 本番コンテナ | 今回の修正前イメージは未診断 | Trivyで修正版のあるHigh/Criticalの指摘0件 |

依存スキャナーの件数は重複する依存経路や開発専用の利用を含み、独立した実環境での攻撃成立件数ではない。SSRFの旧実装の問題はコード点検で確認した。外部DNSを操作する攻撃実験は行っていない。

## 変更内容

- Goの最低バージョンとDockerビルドイメージを1.26.10へ更新。`golang.org/x/net`をv0.60.0へ更新し、関連するcrypto/sys/sync/textも更新した。README・SETUPの必要バージョンを合わせた。
- Capacitor Android/iOSを7.6.9、DOMPurifyを3.4.17、Viteを8.3.5、Vitestを4.1.12以上に更新。lockfileの関連する依存も修正版に更新した。
- Scalarのundici固定値を7.30.1へ更新。Tailwind 3系が使用する`postcss-selector-parser`は7.1.6以上にoverrideし、型チェック・単体テスト・本番ビルドで互換性を確認した。
- 新しいScalar/AI SDKと不整合を起こしていた`@ai-sdk/provider-utils`の全体固定を削除し、各依存が要求する版を使う。更新途中に発生したmissing exportによるビルド失敗は解消した。
- アカウント移行のJSON取得・メディア取得の両方に接続時のIP制限を適用。プライベートIPに加え、共有アドレス空間、ドキュメント用アドレス、IPv4変換・トンネル用の制限対象アドレスも共通の判定で拒否する。リダイレクトを追わない挙動は維持した。
- 開発環境でlocalhostへ移行する場合のみ、`GLIPZ_IDENTITY_TRANSFER_ALLOW_LOOPBACK=true`を明示する。既定値はfalse。localhostは接続時に127.0.0.1へ固定する。公開インスタンスでは有効化しない。`.env.reference.example`に説明を追加した。
- loopback接続の既定拒否と明示許可、予約IPの拒否、リダイレクトの拒否、不正salt/nonce、設定の既定値を回帰テストに追加した。

## 実行した検証

| 検証 | 最終結果 |
| --- | --- |
| Windows `go test -json -count=1 ./...` | 成功。142ケース成功、DB統合テスト1件は環境未指定でスキップ |
| Linux `go test -race -json -count=1 ./...`、専用PostgreSQL・Redisを指定 | 成功。143ケース成功（サブテストを含む）、テストのスキップ0、race報告なし |
| `npm test` | 9ファイル・34テスト成功 |
| `npm run typecheck` | 成功 |
| `npm run build` | 成功 |
| `docker build -f backend/Dockerfile` | Goバックエンド・フロントエンドを含む本番イメージのビルド成功 |
| `python -m unittest discover -s scripts/tests -v` | 運用設定テスト2件成功 |
| `sh -n`（backup-production.sh、init-production.sh） | 成功 |
| Caddy設定の`caddy validate` | 成功 |
| `scripts/security-smoke.py`を診断専用ポートに向けて実行 | OAuthの両grant失効、別利用者の認可、セッション・PAT・feed scope、メール認証、アップロード所有権、メディア公開範囲・GET/HEAD/Range、パスワード保護・更新、ログアウト失効が成功 |
| 最終lockfileの`npm audit --json` | 成功、0件 |
| `govulncheck -show verbose ./...` | 成功、到達可能0件 |
| Trivy 0.76.1（OS・Goバイナリ、HIGH/CRITICAL、ignore-unfixed） | 成功、該当指摘0件 |

DB統合テストの初回は診断用DBへの`infra/postgres/init.sql`投入漏れで失敗した。初期スキーマを投入し、アプリの起動時マイグレーションを適用してから全体を再実行して成功した。診断用のDB・Redis・メール・アプリは専用Composeプロジェクトで隔離し、ホスト公開ポートはloopbackに限定した。

完全ログ・JSONレポートはローカルの`data/security-2026-10-24/`に保存した。このディレクトリはGit管理対象外。

## 残存事項・範囲外

- `GO-2026-5932`は`golang.org/x/crypto/openpgp`の保守停止に関するモジュール情報として残る。Glipzは当該パッケージをインポートせず、govulncheckでも到達可能な問題には該当しない。
- Trivyの集計は修正版があるHigh/Criticalに限定する。全深刻度や未修正版の問題が0件であるとの主張ではない。
- 本番のTLS・DNS・SMTP・クラウド設定、既存データの侵害調査、モバイルネイティブビルド、ブラウザE2E、実際の他インスタンスとの移行・相互運用は検証していない。
- 依存診断と上記のコード点検・回帰検証であり、全機能に脆弱性が存在しないことを保証する包括的監査ではない。

## 参考情報

- [Go脆弱性診断の説明](https://go.dev/doc/security/vuln/)
- [Goの証明書検証に関する修正 GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809)
- [Capacitorの公開アドバイザリ](https://github.com/ionic-team/capacitor/security/advisories/GHSA-rvm3-566m-v7fv)
- [Go HTTP Transportの仕様](https://pkg.go.dev/net/http#Transport)
