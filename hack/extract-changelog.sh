#!/bin/bash -e
die() {
	echo "$@" >&2
	exit 1
}

VERSION="$1"
if [ -z "$VERSION" ]; then
	die "Usage: $0 <version>"
fi

echo "TODO: human readable release note"
echo ""
echo "---"
# Extract the section between "## [v1.2.3] - ..." and the next "## [..."
awk "/## \[$VERSION\] -/{flag=1; next} /## \[/{flag=0} flag" docs/CHANGELOG.md
