# Development recommendations

Before continuing, make sure you've read the [README](./README.md)

## Development tools

This is a (probably incomplete) list of software used to develop the operator:

* [`golang >= v1.13`](https://golang.org/) ...the operator is written in it...
* [`operator-sdk`](https://sdk.operatorframework.io/) Provides all the plumbing and project structure
* [`helm`](https://helm.sh/) Used to deploy and test the operator on a kubernetes cluster
* [`pre-commit`](https://pre-commit.com/) Used to ensure all files are formatted, generated code is up to date and more
* [`gofumpt`](https://github.com/mvdan/gofumpt) Used for code formatting
* [`golangci-lint`](https://github.com/golangci/golangci-lint) Lints for go code

Some additional software you may find useful:

* [`virter`](https://github.com/linbit/virter) can create virtual machines (for example a virtual kubernetes cluster)
* [`cfssl`](https://github.com/cloudflare/cfssl) can create TLS certificates from json configs

### Commit hooks

Commit hooks ensure that all new and changed code is formatted and tested before committing. To set up commit hooks
install `pre-commit` (see above) and run:

```
$ pre-commit install
```

Now, if you try to commit a file that for example is not properly formatted, you will receive a message like:

```
$ git commit -a -m "important fix; no time to check for errors"
Trim Trailing Whitespace.................................................Passed
Fix End of Files.........................................................Passed
Check Yaml...........................................(no files to check)Skipped
Check for added large files..............................................Passed
gofumpt..................................................................Failed
- hook id: gofumpt
- exit code: 1
- files were modified by this hook

version/version.go

golangci-lint............................................................Failed
- hook id: golangci-lint
- exit code: 1
```

## Tests

For basic unit testing use the basic go test framework. If something you want to test relies on the Kubernetes API
(and can't be refactored), you can try to test with [a fake client](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/client/fake?tab=doc).

As of right now, there is no recommended way to end-to-end test the operator. It probably involves some
virtual machines running a basic kubernetes cluster.

## Automated Tests

On every pull request, we run a set of tests, specified in `.github/workflows`. The checks include
* `go test`
* `golandci-lint`
* `pre-commit run`
