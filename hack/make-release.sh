#!/bin/sh -e

YQ="${YQ:-yq}"
KUSTOMIZE="${KUSTOMIZE:-kustomize}"
VERSION="$1"
IMG=${IMG:-quay.io/piraeusdatastore/piraeus-operator}

die() {
	echo "$@" >&2
	exit 1
}

if [ -z "$VERSION" ]; then
	die "Usage: $0 <version>"
fi

# check that version has expected format
# regex taken from https://semver.org/
if ! echo -e "$VERSION" | grep -qP '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$$' ; then
	die "$VERSION does not match format: <maj>.<min>.<patch>"
fi

# check that version does not exist
if git rev-parse --verify --quiet "v$VERSION" ; then
	die "git tag v$VERSION already exists"
fi

# check that working tree is clean
if ! git diff-index --quiet HEAD -- ; then
	die "Refusing to create release from dirty repository"
fi

# replace changelog header "Unreleased" with version and replace link target
sed "s/^## \[Unreleased\]/## [v$VERSION] - $(date +%Y-%m-%d)/" -i ./CHANGELOG.md
sed "s#^\[Unreleased\]: \(.*\)HEAD\$#[v$VERSION]: \\1v$VERSION#" -i ./CHANGELOG.md

# Set image version for kustomize
pushd config/default
$KUSTOMIZE edit set image controller="$IMG:v$VERSION"
popd

# replace deployment instructions in docs
for FILE in ./README.md ./docs/tutorial/get-started.md ; do
	sed -e "s/ref=v2/ref=v$VERSION/" -i "$FILE"
done

# replace chart version+appVersion
$YQ ".version = \"$VERSION\"" -i charts/piraeus/Chart.yaml
$YQ ".appVersion = \"v$VERSION\"" -i charts/piraeus/Chart.yaml

# commit as current release + tag
git commit -aevm "Release v$VERSION" --signoff

# We don't do git tag v$(VERSION) here, as the commit will change once its merged in github
# add "Unreleased" section at top + create comparison link against current master
sed "s/^## \[v$VERSION\]/## [Unreleased]\n\n## [v$VERSION]/" -i CHANGELOG.md
echo "[Unreleased]: https://github.com/piraeusdatastore/piraeus-operator/compare/v$VERSION...HEAD" >> CHANGELOG.md

pushd config/default
$KUSTOMIZE edit set image controller="$IMG:v2"
popd

$YQ ".version = \"$VERSION-dev\"" -i charts/piraeus/Chart.yaml
$YQ ".appVersion = \"v2\"" -i charts/piraeus/Chart.yaml

for FILE in ./README.md ./docs/tutorial/get-started.md ; do
	sed -e "s/ref=v$VERSION/ref=v2/" -i "$FILE"
done

# commit begin of new dev cycle
git commit -aevm "Prepare next dev cycle" --signoff
