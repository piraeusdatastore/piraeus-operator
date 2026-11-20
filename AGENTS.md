# Agent instructions

This repository contains Piraeus Operator, a Kubernetes operator that manages LINSTOR storage clusters.
[CONTRIBUTING.md](./CONTRIBUTING.md) describes the contribution workflow.
Follow it.
The points below cover the steps that most often go wrong in automated changes.

## Branch

Work on the `v2` branch, or on a branch created from `v2`.
The `master` branch contains the legacy operator v1.

## Commands

```sh
make test                  # regenerate code, then run all tests with envtest
make compat-test           # run all tests against the Kubernetes 1.30 API
go test ./pkg/merge/...    # packages without envtest run with plain go test
make lint                  # golangci-lint, pinned version
make manifests             # regenerate CRDs, RBAC, webhook config, chart CRDs
make generate              # regenerate DeepCopy methods
make sync-chart            # copy RBAC and image config into the Helm chart
```

`internal/controller`, `internal/webhook/v1`, and `pkg/k8sgc` use envtest.
`go test` on these packages fails unless `KUBEBUILDER_ASSETS` is set.
After `make test` has downloaded the binaries, run:

```sh
export KUBEBUILDER_ASSETS="$(bin/setup-envtest use -i --bin-dir bin -p path)"
go test ./internal/controller/ -ginkgo.focus '<description>'
```

## Generated files

Do not edit generated files by hand.
Change the source, then regenerate:

* `api/v1/zz_generated.deepcopy.go`: run `make generate`.
* `config/crd/bases/`, `config/rbac/role.yaml`, `config/webhook/manifests.yaml`, and `charts/piraeus/templates/crds.yaml`: change the types in `api/v1` or the `//+kubebuilder` markers, then run `make manifests`.
* RBAC rules and image configuration in `charts/piraeus/`: change `config/rbac/` or `config/manager/`, then run `make sync-chart`.

Commit the regenerated files with the change that caused them.

## API compatibility

`api/v1` is the only API version, and there is no conversion.
Add new fields as optional, and keep the previous behavior when they are not set.
Do not remove or rename fields, change their type or meaning, or add validation that rejects resources the previous release accepted.

## Changelog and upgrade notes

Add an entry under `[Unreleased]` in `docs/CHANGELOG.md` for every change that affects users of the operator.
Describe the effect on the user in one or two lines.
Changes to documentation, CI, tests, or `config/extras/` do not get an entry.

If a change might break an existing deployment, or requires an action from the user, also add a section to `docs/upgrade/README.md`.

## Documentation

* Add new pages to the `nav` section in `mkdocs.yml`.
* Indent nested list items by 4 spaces, and leave a blank line before a list that follows a paragraph.
* Document a discouraged feature with a reference entry and a warning only.
  Do not add examples or how-to guides for it.

## Tests

Do not add tests for the static base manifests under `pkg/resources/`.

## Commits

* Write the subject line in the imperative mood, without a prefix.
* Sign off every commit with `git commit -s`.
* During a review, fold follow-up fixes into the commits that introduced the problem with `git commit --fixup` and `git rebase --autosquash`.

The pre-commit hooks run `make test`, so `git commit` takes several minutes, even for documentation changes.
Run it with a timeout of at least 10 minutes, or in the background.
If a commit command times out, check `git log` before you assume that the commit failed.
