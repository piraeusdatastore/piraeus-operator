# Contributing to Piraeus Operator

This guide describes the workflow for contributing to Piraeus Operator.
For an overview of the project, read the [README](./README.md) first.

## Before you start

Look through the [issues](https://github.com/piraeusdatastore/piraeus-operator/issues) for something to work on.
To contribute something that has no issue yet, open an issue first to discuss it.

## Target branch

Open pull requests against the `v2` branch.
The `master` branch contains the legacy Piraeus Operator v1.

## Development tools

* [Go](https://go.dev/), in the version declared in `go.mod`.
* [`operator-sdk`](https://sdk.operatorframework.io/) provides the project structure and plumbing.
* [`pre-commit`](https://pre-commit.com/) runs the checks described in [Commit hooks](#commit-hooks).
* [`gofumpt`](https://github.com/mvdan/gofumpt) formats Go code.
* [`golangci-lint`](https://github.com/golangci/golangci-lint) lints Go code.
  Run it with `make lint`, which downloads the version pinned in `.golangci-lint-version` to `bin/`.
  CI uses the same version.
* Python 3, [`mkdocs-material`](https://squidfunk.github.io/mkdocs-material/), and [`mike`](https://github.com/jimporter/mike) build the documentation.
  Install them with `pip install 'mkdocs-material[imaging]' mike`.

The Makefile downloads `controller-gen`, `kustomize`, `yq`, and `setup-envtest` to `bin/` when a target uses them.

## Generated files

The `config/` directory is the source of truth for CRDs, RBAC rules, webhook configuration, and the default image configuration.
The following files are generated.
Do not edit them by hand:

| Generated files                                         | Source                                         | Regenerate with |
|---------------------------------------------------------|------------------------------------------------|-----------------|
| `api/v1/zz_generated.deepcopy.go`                       | Types in `api/v1`                              | `make generate` |
| `config/crd/bases/`, `config/rbac/role.yaml`, `config/webhook/manifests.yaml`, `config/manifests/csv-resource-patch.yaml`, `charts/piraeus/templates/crds.yaml` | Types in `api/v1` and `//+kubebuilder` markers | `make manifests` |
| RBAC rules and image configuration in `charts/piraeus/` | `config/rbac/` and `config/manager/`           | `make sync-chart` |

Commit the regenerated files together with the change that caused them.

`make deploy` sets the image in `config/default/kustomization.yaml`.
Do not commit that change.

## API compatibility

The custom resources in `api/v1` are served as `piraeus.io/v1`.
It is the only API version, and there is no conversion between versions.
Existing `LinstorCluster`, `LinstorSatelliteConfiguration`, and `LinstorNodeConnection` resources must keep working after an upgrade without changes:

* Add new fields as optional.
  If a new field is not set, the operator keeps the previous behavior.
* Do not remove or rename fields, and do not change the type or meaning of a field.
* Do not add validation, in the CRD schema or in a webhook, that rejects resources the previous release accepted.

CI runs the test suite against the Kubernetes 1.30 API with `make compat-test`.
Do not use Kubernetes API features that version 1.30 does not have.

## Changelog and upgrade notes

`CHANGELOG.md` follows the [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format.
For every change that affects users of the operator, add an entry to the matching subsection, for example `Added`, `Changed`, or `Fixed`, under `[Unreleased]`.
Describe the effect on the user in one or two lines, not the implementation.
Changes to documentation, CI, tests, or the examples in `config/extras/` do not get an entry.

If a change might break an existing deployment, or requires an action from the user before or after the upgrade, also add a section to the [upgrade notes](./docs/upgrade/README.md).
Name the section after the upgrade it applies to, for example `Upgrades from v2.11 to v2.12`.
Describe what changes, which deployments are affected, and the steps to take.

## Documentation

The documentation in `docs/` is published with MkDocs.
It is organized into tutorials, how-to guides, explanations, and reference pages.
When you add a page, also add it to the `nav` section in `mkdocs.yml`.

MkDocs and GitHub render nested lists differently.
Indent nested list items by 4 spaces, and leave a blank line between a paragraph and a list that follows it.
The `check-docs-lists` commit hook reports lists that render differently on the two platforms.

## Commits

* Write the subject line in the imperative mood, for example `Add option to set the replica count`.
  There is no required prefix.
* Keep each commit to one logical change, including the files regenerated for it.
* Sign off every commit.

This repository enforces the [Developer Certificate of Origin](https://developercertificate.org/).
Every commit needs a `Signed-off-by` line.
Add it by passing the `-s` flag to `git commit`:

```
$ git commit -s -m 'Add option to set the replica count'
```

### Commit hooks

The commit hooks check that new and changed files are formatted, generated files are up to date, and the tests pass.
To set up the commit hooks, install `pre-commit` and run:

```
$ pre-commit install
```

The hooks in `.pre-commit-config.yaml` run the following checks:

* Remove trailing whitespace, fix line endings at the end of files, check YAML syntax, and reject large files.
* Format Go code with `gofumpt`.
* Run `make manifests` to regenerate CRDs, RBAC rules, and webhook configuration.
* Run `make lint` on code changed since `HEAD`.
* Run the full test suite with `make test`.
* Run `make sync-chart` to update the Helm chart.
* Build the documentation with `mkdocs build -s`.
* Run `tools/check-docs-lists.py` on changed files in `docs/`.

Because the hooks run the full test suite, a commit takes several minutes, even if it changes only documentation.

If a hook fails or modifies files, the commit is aborted.
Review the changes, add them, and commit again:

```
$ git commit -s -m 'Add option to set the replica count'
Trim Trailing Whitespace.................................................Passed
Fix End of Files.........................................................Passed
Check Yaml...........................................(no files to check)Skipped
Check for added large files..............................................Passed
gofumpt..................................................................Failed
- hook id: gofumpt
- files were modified by this hook
...
```

## Tests

`make test` regenerates code and manifests, runs `go fmt` and `go vet`, and runs `go test ./...`.
Before it runs the tests, it downloads the control plane binaries (`kube-apiserver` and `etcd`) for [envtest](https://book.kubebuilder.io/reference/envtest).
The Kubernetes version of these binaries matches the version of `k8s.io/api` in `go.mod`.
`make compat-test` runs the same tests against the Kubernetes 1.30 API.

The controller and webhook tests use envtest with Ginkgo and Gomega.
The test suites are set up in [`internal/controller/suite_test.go`](./internal/controller/suite_test.go) and [`internal/webhook/v1/webhook_suite_test.go`](./internal/webhook/v1/webhook_suite_test.go).
`pkg/k8sgc` also uses envtest.
Use these suites for tests that need the Kubernetes API.
Other packages, for example `pkg/merge` and `api/v1`, use plain Go tests with `testify`.

Packages without envtest run with `go test`:

```
$ go test ./pkg/merge/...
```

Packages with envtest fail unless the `KUBEBUILDER_ASSETS` environment variable points to the control plane binaries.
After `make test` has downloaded the binaries, set the variable and select tests with a Ginkgo focus:

```
$ export KUBEBUILDER_ASSETS="$(bin/setup-envtest use -i --bin-dir bin -p path)"
$ go test ./internal/controller/ -ginkgo.focus 'LinstorCluster'
```

The `-i` flag selects the newest version already in `bin/` without querying for new versions.

There is no recommended setup for end-to-end tests.
To try a change on a cluster, use the cluster configured in `~/.kube/config`:

* `make install` installs the CRDs.
  `make run` then runs the operator on your host.
* `make deploy IMG=<image>` deploys the operator with your image into the cluster.

## Automated checks

On every pull request, the workflows in `.github/workflows` run the following checks:

* `golangci-lint`
* `pre-commit run` on the changed files
* `make compat-test`
* `mkdocs build -s`

## Pull request review

Keep the commit history clean during the review.
Fold follow-up fixes into the commits that introduced the problem instead of adding separate commits that address review comments.
For example, create a fixup commit, squash it into its target, and update the pull request:

```
$ git commit -s --fixup=<commit>
$ git rebase -i --autosquash <commit>~
$ git push --force-with-lease
```
