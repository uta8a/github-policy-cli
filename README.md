# github-policy-cli

GitHubリポジトリの設定をTOMLで定義し、`repo-policy plan`で差分を確認、`repo-policy apply`で適用するGo CLIです。

## サポート範囲

**GitHub.com / GitHub Freeの個人アカウントが自身で所有するPublic・Privateリポジトリ**を対象とします。Organization所有、他ユーザー所有のCollaborator参加リポジトリ、GitHub Enterprise Server、Team / Enterprise固有機能は対象外です。アーカイブ済みリポジトリもエラーにします。

| 設定 | Public | Private |
| --- | --- | --- |
| Issues有効化、マージ方式3種、マージ後ブランチ削除 | 対応 | 対応 |
| Wiki有効化・無効化 | 対応 | **指定不可（falseもエラー）** |
| PR作成者制限 all / collaborators_only | 対応 | 対応 |
| Actions有効化、許可ポリシー | 対応 | 対応 |
| Workflow既定権限・PR承認許可 | 対応 | 対応 |

[公式仕様・権限・制約の調査記録](docs/github-support.md)を参照してください。PrivateのWikiは[GitHub Freeでは利用できません](https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis)。Issue作成者の恒久制限、期限付きInteraction limits、ブランチ保護、rulesetsは扱いません。PrivateのActions実行にはFreeプランの利用枠が適用されます。

## ビルドと認証

Go 1.26.7以降（依存go-ghの要件）。

```sh
go build -o repo-policy ./cmd/repo-policy
export GH_TOKEN='YOUR_FINE_GRAINED_PAT'
./repo-policy plan YOUR_LOGIN/YOUR_REPO --config policy.toml
```

Fine-grained PATのResource ownerに自分の個人アカウント、対象リポジトリを選択し、Repository permissionsの **Administration: Read and write / Metadata: Read** を付与してください。planのみでもActions設定を読む場合はAdministration: Readが必要です。`GH_TOKEN`または`gh auth login`の保存済み認証をgo-gh経由で使用します。トークンをTOMLに書かないでください。`GH_HOST`は未指定または`github.com`のみ許可します。

`--all`はトークンから見える自分所有のPublic / Privateを全ページ取得します。PATで選択されていないPrivateリポジトリは発見できません。アカウントの有料プランに応じて機能を増やすことはしません。

## 設定

初期値はカレントディレクトリの`policy.toml`。両Visibilityで利用できる例は[examples/policy.toml](examples/policy.toml)です。

```toml
[repository]
has_issues = true
delete_branch_on_merge = true
allow_squash_merge = true
allow_merge_commit = false
allow_rebase_merge = false
# Publicだけを対象とする場合に指定できます:
# has_wiki = false

[pull_requests]
creation_policy = "collaborators_only"

[actions]
enabled = true
allowed_actions = "all" # all / local_only / selected

[actions.workflow]
default_workflow_permissions = "read" # read / write
can_approve_pull_request_reviews = false
```

未指定項目は変更しません。`false`は明示的な指定です。未知キー・型不一致・不正な列挙値はエラーになります。`selected`はGitHubで設定済みの許可リストを使い、パターン編集は行いません。マージ方式は少なくとも1種類を有効にします。Actions無効状態で許可ポリシーやWorkflowを管理する場合は`enabled = true`も指定してください。PR作成制限はPR機能自体を有効化しません。

## コマンド

```sh
./repo-policy plan owner/repo
./repo-policy apply owner/repo --config policy.toml
./repo-policy plan --all
./repo-policy apply --all
```

`owner/repo`と`--all`は排他です。planは読み取り専用で、差分なしは`No changes.`、正常時は差分の有無によらず終了コード0です。applyは毎回最新状態で計画を作り直し、差分のあるAPIだけ実行します。確認プロンプトはありません。

全対象の設定・所有者・Visibility・現在値を変更前に検証し、エラーがあれば一切更新しません。実行開始後のAPI失敗は対象とHTTPステータスを表示し、独立した処理と残りのリポジトリを継続します。API操作単位の成功・失敗件数を表示し、失敗時は非0で終了します。ロールバックはなく、並行編集による競合も排除できません。

## 開発

[AGENTS.md](AGENTS.md) / [設計](docs/design.md) / [API調査](docs/github-support.md)

```sh
go test ./...
go vet ./...
go build ./cmd/repo-policy
```

テストはFake / httptestで行い、本番GitHubへの変更を実行しません。
