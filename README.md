# github-policy-cli

GitHubリポジトリの設定を宣言し、`repo-policy plan`で差分を確認、`repo-policy apply`で適用するGo CLIです。

## サポート範囲

**GitHub.com / GitHub Freeの個人アカウントが自身で所有するPublic・Privateリポジトリ**を対象とします。Organization所有、他ユーザー所有のCollaborator参加リポジトリ、GHES、Team / Enterprise固有機能は対象外です。アーカイブ済みリポジトリもエラーにします。

| 設定 | Public | Private |
| --- | --- | --- |
| Issues有効化、Projects、マージ方式、ブランチ更新提案・自動削除 | 対応 | 対応 |
| Wiki有効化・無効化 | 対応 | 指定不可（falseもエラー） |
| PR作成者制限 all / collaborators_only | 対応 | 対応 |
| Release immutability | 対応 | 対応 |
| Actions有効化・許可ポリシー・SHA固定・Workflow権限 | 対応 | 対応 |
| Actions外部コントリビューター承認ポリシー | 対応 | 指定不可 |
| Dependabot security updates | 対応 | 対応 |
| main保護ruleset | 対応 | 指定不可（GitHub Freeの制約） |
| Issue作成者の恒久制限 | 手動設定 | 手動設定 |

[公式仕様・権限・制約の調査記録](docs/github-support.md)を参照してください。Privateの[Wiki](https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis)と[rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets)はGitHub Freeでは利用できません。アカウントが有料プランでもFreeの制約を緩和しません。

## 個人向けデフォルト

**`--config`省略時は、取得したVisibilityに応じて組み込み設定を自動選択します。ローカルの`policy.toml`は自動では読みません。** Public / Privateが混在する`--all`でも使用できます。

[Private用設定](internal/policy/defaults/private.toml)：

- Release immutability: on（今後公開するリリースに適用）
- Projects: off
- PR permissions: collaborators only
- Merge commitのみ許可（squash / rebase: off）
- Always suggest updating pull request branches: on
- Automatically delete head branches: on
- Require actions to be pinned to a full-length commit SHA: on
- Dependabot security updates: off（alerts、version updates、他のAdvanced Security設定は変更しません）

Issue permissionsのcollaborators onlyは[画面上で設定できます](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/disabling-issues)が、公式RESTの取得・更新フィールドは確認できていません。**Settings > General > Issues > Collaborators onlyを手動設定してください。** Private用デフォルトのplan / applyにもこの未自動化項目を表示します。`issues.creation_policy`を明示指定すると理由付きエラーになります。期限付きInteraction limitsで代替しません。

[Public用設定](internal/policy/defaults/public.toml)：

- Ruleset名`main`、active、対象は`~DEFAULT_BRANCH`（デフォルトブランチに追従）
- Restrict deletions / Block force pushes / Require signed commits: on
- Require a pull request before merging: on、Required approvals: 0、Allowed merge methods: Merge
- RepositoryのMerge commitも有効化（他ブランチのsquash / rebase可否は維持）
- Actions: Require approval for all external contributors

新規rulesetにbypass actorは設定しません。同名の既存`main` rulesetがある場合は指定部分を更新し、追加ルール、他のPRパラメーター、bypass actorは保持します。他のrulesetは変更しません。既存ルールとの組み合わせによっては、実効制限がこの設定より厳しくなります。同名が複数、branch以外、現在値を取得できない場合は変更前エラーです。既存の例外も除去したい場合はGitHub上で確認してください。

Private向け設定をPublicに継承しません。例えばPublicのPR作成者は制限しません。Actionsを無効にしているPublicリポジトリでは、承認ポリシーの適用はエラーにします。自動でActionsを有効化しません。

## ビルドと認証

Go 1.26.7以降（依存go-ghの要件）。

```sh
go build -o repo-policy ./cmd/repo-policy
export GH_TOKEN='YOUR_FINE_GRAINED_PAT'
./repo-policy plan YOUR_LOGIN/YOUR_REPO
```

Fine-grained PATのResource ownerに自分の個人アカウント、対象リポジトリを選択し、Repository permissionsの **Administration: Read and write / Metadata: Read** を付与してください。読み取り専用のplanには通常Administration: Readで足りますが、既存rulesetのbypass actorが応答から省略される場合は、完全な現在値を取得できる権限が必要です。

`GH_TOKEN`または`gh auth login`の保存済み認証をgo-gh経由で使用します。トークンをTOMLに書かないでください。`GH_HOST`は未指定または`github.com`のみ許可します。`--all`はトークンから見える自分所有のPublic / Privateを全ページ取得します。PATで選択されていないPrivateリポジトリは発見できません。

## 明示的なTOML設定

`--config PATH`を指定すると組み込みデフォルトを使わず、**ファイルで指定した項目だけ**管理します。ファイルが存在しなければエラーです。`false`と未指定は区別します。最小例は[examples/policy.toml](examples/policy.toml)です。組み込みのTOMLもコピーして編集できます。

```sh
./repo-policy plan owner/repo --config policy.toml
./repo-policy apply owner/repo --config policy.toml
```

追加したキーの例：

```toml
[repository]
has_projects = false
allow_update_branch = true

[releases]
immutable = true

[security]
dependabot_security_updates = false

[actions]
sha_pinning_required = true
# Publicのみ。first_time_contributors_new_to_github / first_time_contributors も使用可能。
# fork_pr_approval_policy = "all_external_contributors"

# Publicのみ。trueで上記main保護プロファイルを管理。falseによる削除は非対応。
# [rulesets]
# main_protection = true
```

既存キーは`repository.has_issues` / `has_wiki` / `delete_branch_on_merge` / `allow_merge_commit` / `allow_squash_merge` / `allow_rebase_merge`、`pull_requests.creation_policy`（all / collaborators_only）、`actions.enabled` / `allowed_actions`（all / local_only / selected）、`actions.workflow.default_workflow_permissions`（read / write）/ `can_approve_pull_request_reviews`です。

未知キー・型不一致・不正な列挙値はエラーです。`selected`はGitHubで設定済みの許可リストを使い、パターン編集は行いません。少なくとも1つのマージ方式を有効にしてください。SHA固定はActions無効時にも設定できます。許可ポリシー・Workflow・外部コントリビューター承認はActionsを有効にする必要があります。PR作成制限はPR機能自体を有効化しません。

## コマンドと安全性

```sh
./repo-policy plan owner/repo
./repo-policy apply owner/repo
./repo-policy plan --all
./repo-policy apply --all
```

`owner/repo`と`--all`は排他です。planは読み取り専用で、差分なしは`No changes.`、正常時は差分の有無によらず終了コード0です。applyは毎回最新状態で計画を作り直し、差分のあるAPIだけ実行します。確認プロンプトはありません。

全対象の設定・所有者・Visibility・現在値を変更前に検証し、エラーがあれば一切更新しません。実行時のAPI失敗は対象とHTTPステータスを表示し、独立した処理と残りのリポジトリを継続します。Actions有効化に失敗した後のWorkflow / 承認設定と、Merge commit有効化に失敗した後のrulesetはスキップします。API操作単位の成功・失敗・スキップ件数を表示し、失敗時は非0で終了します。ロールバックはなく、並行編集による競合も排除できません。

## 開発

[AGENTS.md](AGENTS.md) / [設計](docs/design.md) / [API調査](docs/github-support.md)

```sh
go test ./...
go vet ./...
go build ./cmd/repo-policy
```

テストはFake / httptestで行い、本番GitHubへの変更を実行しません。
