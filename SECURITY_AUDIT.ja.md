# セキュリティ監査（2026-09-19）

対象: `83932f9de710410eed693bd65c511bfe250feecf`、D:\glipz。ソースコード、依存関係、ローカル単体テストを調査した。本番環境の設定・ネットワーク・実データ・実際のコンテナイメージは未検査。アプリケーションコードの修正は行っていない。

## 優先して対応する事項

### 1. 高 / P1: Web Pushの送信先を利用したSSRF

- 根拠: `backend/internal/httpserver/webpush.go:275` はendpointと鍵の空文字のみを検査して保存する。`:60` の送信では制限付きHTTPクライアントを指定していない。
- 条件: Web PushのVAPID設定が有効で、攻撃者が一般ユーザーとしてログインできること。自身の購読情報に任意のendpointと正しい形式の鍵を登録し、別アカウントからフォロー等で通知を発生させられる。
- 影響: バックエンドからlocalhost・プライベートIP等へのHTTPリクエストが可能。レスポンス本文を利用者に返す経路は確認していないため、ブラインドSSRFとして扱う。送信本文はWeb Push形式であり、任意の本文を自由に送れるという意味ではない。
- 検証: 実装と同じライブラリ・Optionsを使うローカルテストで、テスト用127.0.0.1サーバーへのPOSTを確認。DBを含む完全なAPI経由の再現は未実施。
- 修正: HTTPSと許可されたPushサービスを検証し、接続時のIP検査とリダイレクト検査を行うHTTPClientを指定する。既存の`newPublicOutboundHTTPClient`も活用できるが、Push専用の宛先制限が望ましい。
- 一次資料: [webpush-go v1.4.0の送信実装](https://raw.githubusercontent.com/SherClockHolmes/webpush-go/v1.4.0/webpush.go)。HTTPClient未指定時は標準クライアントを生成する。

### 2. 高 / P1: メディアURLを知っていれば投稿の閲覧制限を迂回できる

- 根拠: `backend/internal/httpserver/server.go:159` はメディアを認証なしで公開。`backend/internal/httpserver/public_media.go:336` はSec-Fetch-Dest等を検査するだけで、投稿のvisibility・パスワード解除・会員資格・所有者を確認しない。その後ストレージから元データを返す。
- 条件: 有効なオブジェクトURLを知っていること。UUIDの総当たりは前提にしない。例えば公開時のURLを保存してから投稿をprivateに変更しても、同じURLにアクセスできる。
- 影響: 非公開・フォロワー限定・パスワード付き・会員限定メディアについて、URLが知られた後のアクセス制御が働かない。`Sec-Fetch-Dest: image`はHTTPクライアントで自由に送れるため、ダミーファイルへの置換は権限管理にならない。
- 検証: 既存の`TestHandlePublicMediaObjectReturnsDecoyForDirectDownloads`が、認証なしで同ヘッダーを付けるだけで元のデータを返すことを確認している。このテストも実行して成功した。投稿の状態変更を含むDB統合テストは未実施。
- 修正: 公開アセットと保護対象を分け、保護対象は投稿の閲覧権限を配信時に検査するか、認可後に短期間だけ使える署名付きURLを発行する。公開CDN/S3の直接アクセスも同様に制限する。ローカル保存の`public, max-age=31536000, immutable`も保護対象には適用しない。

### 3. 中 / P2: ログアウトしてもアクセストークンが失効しない

- 根拠: `backend/internal/httpserver/server.go:850` はCookieを削除するだけ。`:354` の認証にはセッション失効照合がない。ログイン時のトークン有効期間は`:756`と`:831`で24時間。
- 条件・影響: コピー済み／漏えい済みのトークンを持つ相手は、利用者がログアウトしても期限まで認証可能。これはトークンを盗む方法自体を発見したという指摘ではない。
- 検証: トークン発行→同Cookie付きでhandleLogout呼び出し→同トークンをprincipalForAccessへ渡すローカルテストで、引き続き認証されることを確認。
- 修正: サーバー管理のセッションIDまたはjtiを導入し、ログアウト時に失効させる。短命アクセストークンと失効可能な更新トークンの構成も検討する。

### 4. 中 / P2: Web Pushの15秒タイムアウトが送信に適用されない

- 根拠: `backend/internal/httpserver/webpush.go:38` は通知ごとにgoroutineを作り、`:39` で15秒のcontextを生成するが、`:60` は`SendNotificationWithContext`ではなく`SendNotification`を呼ぶ。HTTPClientのTimeoutも未指定。
- 影響: 応答しないendpointへの接続が長時間残り、通知を繰り返すことでgoroutine・接続等が蓄積する。購読件数制限・送信並列数制限も当該経路では確認できない。可用性への実際の負荷試験は行っていない。
- 修正: contextを送信関数へ渡し、HTTPClient.Timeoutを設定する。送信キューの並列数・購読件数・通知頻度を制限する。宛先を公開IPに限定するだけではこの問題は解消しない。

## 依存関係の検出結果

`npm audit --json --ignore-scripts`: 既知の脆弱性0件。これはアプリケーション固有の欠陥がないことを意味しない。

`govulncheck`は、ローカルのGo 1.26.5で12件の脆弱性について呼出し経路を検出した。到達可能性は静的解析の結果であり、12件すべての実攻撃が成立したという意味ではない。全出力は[security-govulncheck.txt](security-govulncheck.txt)に保存した。

| 対象 | スキャン時の版 | 検出事項を修正する版 | 補足 |
|---|---|---|---|
| Go標準ライブラリ | 1.26.5 | 1.26.6以上 | URL処理、TLS、HTTP、XML、ASN.1等。Dockerfileはさらに古い1.26.2を固定 |
| golang.org/x/text | 0.36.0 | 0.39.0以上 | 不正入力による無限ループ |
| golang.org/x/net | 0.53.0 | 0.55.0以上 | IDNAラベル検証 |
| github.com/go-chi/chi/v5 | 5.2.5 | 5.3.0以上 | IPヘッダー偽装関連3件。TrustProxyHeadersが有効な場合にRealIPを使用 |
| AWS SDK service/s3 | 1.58.2 | 1.97.3以上 | EventStream関連。eventstream側は1.7.8以上。SDK一式の整合性を保って更新 |
| github.com/jackc/pgx/v5 | 5.6.0 | 5.9.2以上 | SQL注入の成立には非デフォルトのsimple protocolと特定のSQL構文が必要 |

Goの[GO-2026-6218](https://pkg.go.dev/vuln/GO-2026-6218)はURL解決処理の計算量に関する問題で、1.26.6で修正される。pgxの[GO-2026-5004](https://pkg.go.dev/vuln/GO-2026-5004)について、このコード内ではsimple protocolを明示的に有効化する設定を発見していない。DATABASE_URL等の実運用設定は未確認であり、SQL注入が成立すると断定しない。chiについても、バックエンドの隔離とプロキシによるヘッダー上書きで条件が変わる。

まずSSRF・保護対象メディア配信を修正し、Go/Dockerビルドと依存関係を更新する。その後、セッション失効とPush送信の資源制限を実装することを推奨する。

## 検証結果と範囲

- `go test ./...`: 全テスト成功。
- `npm test`: 7ファイル、22テスト成功。
- 一時検証テスト: Pushのlocalhost送信、ログアウト後の同トークン受理を確認。再現用コードは[security-audit-probe_test.go.txt](security-audit-probe_test.go.txt)に保存。通常のテスト対象には含めていない。
- 認証、OAuthスコープ、CSRF、外向きHTTP、HTMLサニタイズ、メディア、設定、依存関係の主要経路を確認。全API・全連合イベントの網羅的証明、Git履歴全体の秘密情報スキャン、本番侵入試験は対象外。
- 本番サービスへの攻撃、内部ネットワーク探索、実データ取得は実施していない。
