# github-policy-cli

> Declare, review, and apply GitHub repository settings.

A Go CLI for public and private repositories owned by your personal GitHub.com account, using GitHub Free capabilities.

- Review settings with a read-only `plan` before applying changes.
- Use built-in public/private presets or an explicit TOML policy.
- Manage one repository or all repositories visible to your token.
- Validate every target before writing, and update only changed settings.

## Quick start

Requires Go 1.26.7 or later.

Create a token in [GitHub's fine-grained PAT settings](https://github.com/settings/personal-access-tokens) with your personal account as resource owner, the intended repositories selected, and **Administration: Read and write / Metadata: Read** permissions.

**This token grants powerful repository administration permissions. Delete it from GitHub's token settings after use.**

```sh
go build -o repo-policy ./cmd/repo-policy
export GH_TOKEN='YOUR_FINE_GRAINED_PAT'
./repo-policy plan YOUR_LOGIN/YOUR_REPO
./repo-policy apply YOUR_LOGIN/YOUR_REPO
```

Without `--config`, commands use the repository's public/private preset. Review the plan before applying: `apply` writes changes without a confirmation prompt. Both presets require you to configure **Settings > General > Issues > Collaborators only** manually.

[Read the user guide](docs/usage.md) for authentication, presets, configuration, supported settings, and execution behavior.

## Documentation

- [User guide](docs/usage.md)
- [GitHub support and API permissions](docs/github-support.md)
- [Design](docs/design.md)
- [Versioning and releases](docs/versioning.md)

## Contributing

See the [development guide](docs/development.md) for checks, repository conventions, and CI.

## License

[Apache-2.0](LICENSE).
