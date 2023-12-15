#!/usr/bin/env bash

set -eo pipefail

if [ -n "$V" ]; then
	set -x
fi

# Change to the root of the repository
if [ -n "${BASH_SOURCE[0]}" ]; then
	SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
	cd "$SCRIPT_DIR/../../.."
fi

# Print the provided error message and exit with exit-code set to 1
die() {
	echo "$@" >&2
	exit 1
}

# Read the json value from STDIN and format it as a patch for kustomization to STDOUT
#
# Takes three parameters:
# * The kind of the resource that is patched
# * The name of the resource that is patched
# * The jq expression that converts STDIN into either a strategic merge patch (i.e. partial Kubernetes resource) or
#   a RFC 6902 JSON patch
#
# Example:
# $ echo '{"replicas": 3}' | format_patch Deployment example '{"spec": {"replicas" .replicas}}'
# {
#  "target": {
#    "kind": "Deployment",
#    "name": "example"
#  },
#  "patch": "{\"spec\":{\"replicas\":3}}"
#}
format_patch() {
	local KIND="$1"
	local NAME="$2"
	local EXPR="$3"
	jq --arg KIND "$KIND" --arg NAME "$NAME" "$EXPR | {\"target\": {\"kind\": \$KIND, \"name\": \$NAME}, \"patch\": (. | tostring) }"
}

# Append the kustomize patch from STDIN to the kustomize configuration in the provided directory.
# This will write the updated kustomization.yaml to STDOUT
append_patch() {
	jq -s --slurpfile KUSTOMIZATION "$1/kustomization.yaml" '. as $INPUT | $KUSTOMIZATION[0] | .patches += $INPUT'
}

# Compare the kustomization read from STDIN to the one provided in the first argument.
#
# This will print a diff of the read kustomization to the one read from the provided directory.
#
# The optional second argument is used when displaying the patch: instead of diffing the output of the two
# kustomizations, diff the result of applying the patches to the Operator resources.
#
# Afterwards, there will be a temporary "KUSTOMIZATION.tmp", holding an updated copy of the directory provided in
# the first argument.
diff_patch() {
	local KUSTOMIZATION="$1"
	local OPERATOR_RESOURCE="$2"

	# Make of copy of the existing kustomization and update it with the new value read from STDIN.
	cp -a "$KUSTOMIZATION" "$KUSTOMIZATION.tmp"
	cat > "$KUSTOMIZATION.tmp/kustomization.yaml"

	if [ -n "$OPERATOR_SOURCE" ] && [ -n "$OPERATOR_RESOURCE" ]; then
		# The patch we got only adds a "spec.patches" entry on the resulting resource: Instead of displaying the additional
		# entry, we show the result of applying this added patch to the Operator resources
		mkdir "$KUSTOMIZATION.tmp/old" "$KUSTOMIZATION.tmp/new"
		# Create new kustomizations, using only the .spec.patches and apply them to the raw Operator resources
		jq -r '.patches[].patch' "$KUSTOMIZATION/kustomization.yaml" \
		  | jq '.[] | select(.path == "/spec/patches/-") | .value' \
		  | jq --slurp --arg RESOURCES "../../../$OPERATOR_SOURCE/$OPERATOR_RESOURCE" '{"resources": [$RESOURCES], "patches": .}' > "$KUSTOMIZATION.tmp/old/kustomization.yaml"
		jq -r '.patches[].patch' "$KUSTOMIZATION.tmp/kustomization.yaml" \
		  | jq '.[] | select(.path == "/spec/patches/-") | .value' \
		  | jq --slurp --arg RESOURCES "../../../$OPERATOR_SOURCE/$OPERATOR_RESOURCE" '{"resources": [$RESOURCES], "patches": .}' > "$KUSTOMIZATION.tmp/new/kustomization.yaml"
		# Render the result of comparing the kustomizations on the Operator resources
		diff --unified --label "Default resource" --label "Updated resource" --show-function-line='^kind:' <(kubectl kustomize "$KUSTOMIZATION.tmp/old") <(kubectl kustomize "$KUSTOMIZATION.tmp/new") || true
	else
		# Just render the new kustomization and compare it to the existing one
		diff --unified --label "Default resource" --label "Updated resource" --show-function-line='^kind:' <(kubectl kustomize "$KUSTOMIZATION") <(kubectl kustomize "$KUSTOMIZATION.tmp") || true
	fi
}

# Ask the user to confirm or discard a proposed change
#
# This assumes that diff_patch was called previously, so in addition to the directory provided in the first argument,
# there is the updated copy located at "$DIRECTORY.tmp".
#
# The second argument is the default argument if the user does not provide any input, either "n" or "y".
confirm_patch() {
	if [ "$2" == y ]; then
		YN="Y/n"
	else
		YN="y/N"
	fi

	while true; do
		read -rp "Apply the patch ($YN)? " CHOICE < "$READ_FROM" || true
		case "${CHOICE:-$2}" in
			y|Y )
				mv "$1.tmp/kustomization.yaml" "$1/kustomization.yaml"
				break
				;;
			n|N )
				break
				;;
			* )
				;;
		esac
	done

	rm -rf "$1.tmp"
}

# Fetch the sources for the operator so diffing shows useful results
get_sources() {
	if [ -d pkg/resources/cluster/controller ] && [ -d pkg/resources/cluster/csi-controller ] && [ -d pkg/resources/cluster/csi-node ] && [ -d pkg/resources/satellite/pod ]; then
		echo "."
		return
	fi

	if ! command -v curl >/dev/null ; then
		echo "Could not find optional tool \"curl\", cannot show rich diffs." >&2
		return
	fi

	if ! command -v tar >/dev/null ; then
		echo "Could not find optional tool \"tar\", cannot show rich diffs." >&2
		return
	fi

	if curl -fsSL https://github.com/piraeusdatastore/piraeus-operator/archive/refs/heads/v2.tar.gz | tar -xzC "$1" 2>/dev/null ; then
		echo "$1/piraeus-operator-2"
	else
		echo "Could not download operator sources, not showing rich diffs." >&2
	fi
}

### Step 1: Sanity checks

if ! command -v kubectl >/dev/null ; then
	die Could not find required tool \"kubectl\"
fi

if ! command -v jq >/dev/null ; then
	die Could not find required tool \"jq\"
fi

echo "Using kubectl context: $(kubectl config current-context)"

TEMPDIR="$(mktemp -d .v1-information.XXXXXX)"
if [ -n "$KEEP_TMP" ]; then
	echo "TEMPDIR: $TEMPDIR"
else
	trap 'rm -rf "$TEMPDIR"' EXIT
fi

OPERATOR_SOURCE="${OPERATOR_SOURCE:-$(get_sources "$TEMPDIR")}"
READ_FROM=/dev/stdin
if [ ! -t 0 ] && [ -e /dev/tty ]; then
	READ_FROM=/dev/tty
fi

mkdir -p "$TEMPDIR/linstorcluster" "$TEMPDIR/linstorsatelliteconfiguration"

if ! kubectl get linstorcontrollers -A -ojson > "$TEMPDIR/linstorcontrollers.json" ; then
	die "No LinstorController API available, are you sure kubectl points at the right cluster?"
fi

if ! kubectl get linstorsatellitesets -A -ojson > "$TEMPDIR/linstorsatellitesets.json" ; then
	die "No LinstorSatelliteSet API available, are you sure kubectl points at the right cluster?"
fi

if ! kubectl get linstorcsidrivers -A -ojson > "$TEMPDIR/linstorcsidrivers.json" ; then
	die "No LinstorCSIDriver API available, are you sure kubectl points at the right cluster?"
fi

echo "Found LinstorControllers: $(jq -c '.items | map(.metadata.name)' $TEMPDIR/linstorcontrollers.json)"
NR_CONTROLLERS="$(jq '.items | length' $TEMPDIR/linstorcontrollers.json)"
if [ "$NR_CONTROLLERS" -ne 1 ]; then
	die "Expected exactly one LinstorController resource, got $NR_CONTROLLERS"
fi

echo "Found LinstorSatelliteSets: $(jq -c '.items | map(.metadata.name)' $TEMPDIR/linstorsatellitesets.json)"
NR_SATELLITESETS="$(jq '.items | length' $TEMPDIR/linstorsatellitesets.json)"
if [ "$NR_SATELLITESETS" -lt 1 ]; then
	die "Expected at least one LinstorSatelliteSets resource, got $NR_SATELLITESETS"
fi

echo "Found LinstorCSIDrivers: $(jq -c '.items | map(.metadata.name)' $TEMPDIR/linstorcsidrivers.json)"
NR_LINSTOR_CSI_DRIVERS="$(jq '.items | length' $TEMPDIR/linstorcsidrivers.json)"
if [ "$NR_LINSTOR_CSI_DRIVERS" -ne 1 ]; then
	die "Expected exactly one LinstorCSIDriver resource, got $NR_LINSTOR_CSI_DRIVERS"
fi

EXPECTED_CONTROLLER_ENDPOINT="http://$(jq -r '.items[0].metadata.name' "$TEMPDIR/linstorcontrollers.json").$(jq -r '.items[0].metadata.namespace' "$TEMPDIR/linstorcontrollers.json").svc:3370"

jq -n '{"apiVersion": "piraeus.io/v1", "kind": "LinstorCluster", "metadata": {"name": "linstorcluster"}, "spec": {"patches": []}}' > "$TEMPDIR/linstorcluster/linstorcluster.yaml"
jq -n '{"apiVersion": "kustomize.config.k8s.io/v1beta1", "kind": "Kustomization", "resources": ["linstorcluster.yaml"], "patches": []}' > "$TEMPDIR/linstorcluster/kustomization.yaml"
jq -n '{"apiVersion": "kustomize.config.k8s.io/v1beta1", "kind": "Kustomization", "resources": [], "patches": []}' > "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml"
jq -n '{"apiVersion": "kustomize.config.k8s.io/v1beta1", "kind": "Kustomization", "resources": ["linstorcluster", "linstorsatelliteconfiguration"]}' > "$TEMPDIR/kustomization.yaml"

if [ "$(jq '.items[0].spec.additionalEnv | length' "$TEMPDIR/linstorcontrollers.json")" -gt 0 ]; then
	echo "Found additional environment variables passed to LINSTOR Controller"
	jq '.items[0].spec.additionalEnv' "$TEMPDIR/linstorcontrollers.json" \
	  | format_patch Deployment linstor-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-controller"}, "spec": {"template": {"spec": {"containers": [{"name": "linstor-controller", "env": .}]}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/controller
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ "$(jq '.items[0].spec.additionalProperties | length' "$TEMPDIR/linstorcontrollers.json")" -gt 0 ]; then
	echo "Found additional properties configured on LINSTOR Controller"
	jq '.items[0].spec.additionalProperties | to_entries | map({"name": .key, "value": .value})' "$TEMPDIR/linstorcontrollers.json" \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/properties", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster"
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ "$(jq '.items[0].spec.affinity | length' "$TEMPDIR/linstorcontrollers.json")" -gt 0 ]; then
	echo "Found custom affinity set on LINSTOR Controller"
	jq '.items[0].spec.affinity' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch Deployment linstor-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-controller"}, "spec": {"template": {"spec": {"affinity": . }}}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq -r '.items[0].spec.dbConnectionURL' "$TEMPDIR/linstorcontrollers.json")" != "k8s" ]; then
	echo "Found custom LINSTOR Controller Database connection"
	printf '[db]\n  connection_url = "%s"\n' "$(jq -r '.items[0].spec.dbConnectionURL' "$TEMPDIR/linstorcontrollers.json")" \
		| jq --raw-input --slurp . \
		| format_patch ConfigMap linstor-controller-config '{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "linstor-controller-config"}, "data": {"linstor.toml": .}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/controller
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ "$(jq -r '.items[0].spec.extraVolumes | length' "$TEMPDIR/linstorcontrollers.json")" -gt 0 ]; then
	echo "Found extra volumes for LINSTOR Controller Pod"
	jq '.items[0].spec.extraVolumes' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch Deployment linstor-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-controller"}, "spec": {"template": {"spec": {"volumes": . }}}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ -n "$(jq -r '.items[0].spec.linstorHttpsClientSecret' "$TEMPDIR/linstorcontrollers.json")" ] || [ -n "$(jq -r '.items[0].spec.linstorHttpsControllerSecret' "$TEMPDIR/linstorcontrollers.json")" ]; then
	echo "Found LINSTOR Controller HTTPS configuration"
	EXPECTED_CONTROLLER_ENDPOINT="https://$(jq -r '.items[0].metadata.name' "$TEMPDIR/linstorcontrollers.json").$(jq -r '.items[0].metadata.namespace' "$TEMPDIR/linstorcontrollers.json").svc:3371"
	jq '{"clientSecretName": .items[0].spec.linstorHttpsClientSecret, "apiSecretName": .items[0].spec.linstorHttpsControllerSecret}' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/apiTLS", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster"
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ -n "$(jq -r '.items[0].spec.luksSecret' "$TEMPDIR/linstorcontrollers.json")" ]; then
	echo "Found LINSTOR Controller passphrase secret"
	jq '.items[0].spec.luksSecret' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/linstorPassphraseSecret", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster"
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ "$(jq -r '.items[0].spec.sidecars | length' "$TEMPDIR/linstorcontrollers.json")" -gt 0 ]; then
	echo "Found LINSTOR Controller sidecars"
	jq '.items[0].spec.sidecars' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch Deployment linstor-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-controller"}, "spec": {"template": {"spec": {"containers": .}}}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ -n "$(jq -r '.items[0].spec.sslSecret // ""' "$TEMPDIR/linstorcontrollers.json")" ]; then
	echo "Found LINSTOR Controller internal TLS configuration"
	jq '{"secretName": .items[0].spec.sslSecret}' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/internalTLS", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster"
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ -n "$(jq -r '.items[0].spec.tolerations' "$TEMPDIR/linstorcontrollers.json")" ]; then
	echo "Found LINSTOR Controller Pod tolerations"
	jq '.items[0].spec.tolerations' "$TEMPDIR/linstorcontrollers.json" \
		| format_patch Deployment linstor-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-controller"}, "spec": {"template": {"spec": {"tolerations": .}}}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

### Check LinstorCSIDriver

if [ "$(jq '.items[0].spec.controllerAffinity | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found custom affinity set on LINSTOR CSI Controller"
	jq '.items[0].spec.controllerAffinity' "$TEMPDIR/linstorcsidrivers.json" \
		| format_patch Deployment linstor-csi-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-csi-controller"}, "spec": {"template": {"spec": {"affinity": . }}}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq '.items[0].spec.controllerReplicas' "$TEMPDIR/linstorcsidrivers.json")" -ne 1 ] || [ "$(jq '.items[0].spec.controllerStrategy | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found custom replica settings for LINSTOR CSI Controller"
	jq '{"replicas": .items[0].spec.controllerReplicas, "strategy": .items[0].spec.controllerStrategy}' "$TEMPDIR/linstorcsidrivers.json" \
		| format_patch Deployment linstor-csi-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-csi-controller"}, "spec": .}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-controller
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ "$(jq '.items[0].spec.controllerTolerations | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found custom tolerations for LINSTOR CSI Controller"
	jq '.items[0].spec.controllerTolerations' "$TEMPDIR/linstorcsidrivers.json" \
		| format_patch Deployment linstor-csi-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-csi-controller"}, "spec": {"template": {"spec": {"tolerations": .}}}}' \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq '.items[0].spec.extraVolumes | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found additional volumes for LINSTOR CSI Controller"
	jq '.items[0].spec.extraVolumes' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch Deployment linstor-csi-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-csi-controller"}, "spec": {"template": {"spec": {"volumes": .}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq '.items[0].spec.sidecars | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found additional containers for LINSTOR CSI Controller"
	jq '.items[0].spec.sidecars' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch Deployment linstor-csi-controller '{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": "linstor-csi-controller"}, "spec": {"template": {"spec": {"containers": .}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-controller
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq -r '.items[0].spec.kubeletPath' "$TEMPDIR/linstorcsidrivers.json")" != "/var/lib/kubelet" ]; then
	echo "Found non-standard kubelet path"
	jq '.items[0].spec.kubeletPath' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch DaemonSet linstor-csi-node '{"apiVersion": "apps/v1", "kind": "DaemonSet", "metadata": {"name": "linstor-csi-node"}, "spec": {"template": {"spec": {"containers": [{"name": "linstor-csi", "volumeMounts": [{"name": "publish-dir", "mountPath": "/var/lib/kubelet", "$patch": "delete"}, {"name": "publish-dir", "mountPath": ., "mountPropagation": "Bidirectional"}]}]}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-node
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ -n "$(jq -r '.items[0].spec.linstorHttpsClientSecret' "$TEMPDIR/linstorcsidrivers.json")" ]; then
	echo "Found LINSTOR CSI Client HTTPS configuration"
	jq '.items[0].spec.linstorHttpsClientSecret' "$TEMPDIR/linstorcsidrivers.json" \
		| format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/apiTLS/csiControllerSecretName", "value": .}, {"op": "add", "path": "/spec/apiTLS/csiNodeSecretName", "value": .}]' \
		| append_patch "$TEMPDIR/linstorcluster" \
		| diff_patch "$TEMPDIR/linstorcluster"
	confirm_patch "$TEMPDIR/linstorcluster" y
fi

if [ "$(jq '.items[0].spec.nodeAffinity | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found custom affinity set on LINSTOR CSI Nodes"
	jq '.items[0].spec.nodeAffinity' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch DaemonSet linstor-csi-node '{"apiVersion": "apps/v1", "kind": "DaemonSet", "metadata": {"name": "linstor-csi-node"}, "spec": {"template": {"spec": {"affinity": .}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-node
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq '.items[0].spec.nodeExtraVolumes | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found additional volumes for LINSTOR CSI Nodes"
	jq '.items[0].spec.nodeExtraVolumes' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch DaemonSet linstor-csi-node '{"apiVersion": "apps/v1", "kind": "DaemonSet", "metadata": {"name": "linstor-csi-node"}, "spec": {"template": {"spec": {"volumes": .}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-node
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq '.items[0].spec.nodeSidecars | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found additional containers for LINSTOR CSI Nodes"
	jq '.items[0].spec.nodeSidecars' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch DaemonSet linstor-csi-node '{"apiVersion": "apps/v1", "kind": "DaemonSet", "metadata": {"name": "linstor-csi-node"}, "spec": {"template": {"spec": {"containers": .}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-node
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

if [ "$(jq '.items[0].spec.nodeTolerations | length' "$TEMPDIR/linstorcsidrivers.json")" -gt 0 ]; then
	echo "Found custom tolerations for LINSTOR CSI Nodes"
	jq '.items[0].spec.nodeTolerations' "$TEMPDIR/linstorcsidrivers.json" \
	  | format_patch DaemonSet linstor-csi-node '{"apiVersion": "apps/v1", "kind": "DaemonSet", "metadata": {"name": "linstor-csi-node"}, "spec": {"template": {"spec": {"tolerations": .}}}}' \
	  | format_patch LinstorCluster linstorcluster '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
	  | append_patch "$TEMPDIR/linstorcluster" \
	  | diff_patch "$TEMPDIR/linstorcluster" pkg/resources/cluster/csi-node
	confirm_patch "$TEMPDIR/linstorcluster" n
fi

jq -n '{"apiVersion": "piraeus.io/v1", "kind": "LinstorSatelliteConfiguration", "metadata": {"name": "host-networking"}, "spec": {"patches": []}}' > "$TEMPDIR/linstorsatelliteconfiguration/host-networking.yaml"
jq '.resources += ["host-networking.yaml"]' "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml" > "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml.new"
mv "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml.new" "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml"

echo "For compatibility, applying host-networking to all satellites"
jq -n '{}' \
	| format_patch LinstorSatelliteConfiguration "host-networking" '[{"op": "add", "path": "/spec/podTemplate", "value": {"spec": {"hostNetwork": true}}}]' \
	| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
	| diff_patch "$TEMPDIR/linstorsatelliteconfiguration"
confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" y

jq_item() {
	jq ".items[$IDX]" "$TEMPDIR/linstorsatellitesets.json" | jq "$@"
}

for IDX in $(seq 0 "$(jq '.items | length - 1' "$TEMPDIR/linstorsatellitesets.json")"); do
	NAME="$(jq_item -r '.metadata.name')"
	jq -n --arg NAME "$NAME" '{"apiVersion": "piraeus.io/v1", "kind": "LinstorSatelliteConfiguration", "metadata": {"name": $NAME}, "spec": {"patches": [], "storagePools": []}}' > "$TEMPDIR/linstorsatelliteconfiguration/$NAME.yaml"
	jq --arg CONFIG "$NAME.yaml" '.resources += [$CONFIG]' "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml" > "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml.new"
	mv "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml.new" "$TEMPDIR/linstorsatelliteconfiguration/kustomization.yaml"

	if [ "$(jq_item '.spec.affinity?.nodeAffinity?.requiredDuringSchedulingIgnoredDuringExecution | length')" -gt 0 ]; then
		jq_item '.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/nodeAffinity", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration"
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" y
	fi

	if [ "$(jq_item -c '.spec.additionalEnv')" != '[{"name":"LB_SELINUX_AS","value":"modules_object_t"}]' ]; then
		echo "Found additional environment variables passed to LINSTOR Satellite"
		jq_item '.spec.additionalEnv' \
			| format_patch Pod satellite '{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "satellite"}, "spec": {"containers": [{"name": "linstor-satellite", "env": .}]}}' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration" pkg/resources/satellite/pod
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" y
	fi

	if [ "$(jq_item '.spec.dnsPolicy | length')" -gt 0 ]; then
		echo "Found custom DNS policy for LINSTOR Satellite Pod"
		jq_item '.spec.dnsPolicy' \
			| format_patch Pod satellite '{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "satellite"}, "spec": {"dnsPolicy": .}}' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration" pkg/resources/satellite/pod
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" n
	fi

	if [ "$(jq_item '.spec.extraVolumes | length')" -gt 0 ]; then
		echo "Found additional volumes for LINSTOR Satellite Pod"
		jq_item '.spec.extraVolumes' \
			| format_patch Pod satellite '{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "satellite"}, "spec": {"volumes": .}}' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration" pkg/resources/satellite/pod
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" n
	fi

	if [ "$(jq_item '.spec.mountDrbdResourceDirectoriesFromHost')" == "true" ]; then
		echo "Found DRBD config mount for LINSTOR Satellite Pod"
		jq -n '{}' \
			| format_patch Pod satellite '{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "satellite"}, "spec": {"volumes": [{"name": "etc-drbd-conf", "hostPath": {"path": "/etc/drbd.conf", "type": "File"}}, {"name": "etc-drbd-d", "hostPath": {"path": "/etc/drbd.d", "type": "Directory"}}], "containers": [{"name": "linstor-satellite", "volumeMounts": [{"name": "etc-drbd-conf", "mountPath": "/etc/drbd.conf", "readOnly": true}, {"name": "etc-drbd-d", "mountPath": "/etc/drbd.d", "readOnly": true}]}]}}' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration" pkg/resources/satellite/pod
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" n
	fi

	if [ "$(jq_item '.spec.sidecars | length')" -gt 0 ]; then
		echo "Found additional containers for LINSTOR Satellite Pod"
		jq_item '.spec.sidecars' \
			| format_patch Pod satellite '{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "satellite"}, "spec": {"containers": .}}' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration" pkg/resources/satellite/pod
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" n
	fi

	if [ -n "$(jq_item -r '.spec.sslSecret // ""')" ]; then
		echo "Found TLS secret for LINSTOR Satellite"
		jq_item '.spec.sslSecret' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/internalTLS", "value": {"secretName": .}}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration"
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" y
	fi

	if [ "$(jq_item '.spec.tolerations | length')" -gt 0 ]; then
		echo "Found custom tolerations for LINSTOR Satellite Pod"
		jq_item '.spec.tolerations' \
			| format_patch Pod satellite '{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "satellite"}, "spec": {"tolerations": .}}' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/patches/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration" pkg/resources/satellite/pod
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" n
	fi

	if [ "$(jq_item '.spec.storagePools.lvmPools | length')" -gt 0 ]; then
		echo "Found LVM storage pool"
		jq_item '.spec.storagePools.lvmPools[]' \
		  | jq '.devicePaths as $PATHS | .volumeGroup as $VG | {"name": .name, "lvmPool": {}} | if $VG != "" then .lvmPool.volumeGroup = $VG else . end | if $PATHS | length > 0 then .source = {"hostDevices": $PATHS} else . end' \
		  | format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/storagePools/-", "value": .}]' \
		  | append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
		  | diff_patch "$TEMPDIR/linstorsatelliteconfiguration"
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" y
	fi

	if [ "$(jq_item '.spec.storagePools.lvmThinPools | length')" -gt 0 ]; then
		echo "Found LVM THIN storage pool"
		jq_item '.spec.storagePools.lvmThinPools[]' \
			| jq '.devicePaths as $PATHS | .volumeGroup as $VG | .thinVolume as $LV | {"name": .name, "lvmThinPool": {}} | if $VG != "" then .lvmThinPool.volumeGroup = $VG else . end | if $LV != "" then .lvmThinPool.thinPool = $LV else . end | if $PATHS | length > 0 then .source = {"hostDevices": $PATHS} else . end' \
			| format_patch LinstorSatelliteConfiguration "$NAME" '[{"op": "add", "path": "/spec/storagePools/-", "value": .}]' \
			| append_patch "$TEMPDIR/linstorsatelliteconfiguration" \
			| diff_patch "$TEMPDIR/linstorsatelliteconfiguration"
		confirm_patch "$TEMPDIR/linstorsatelliteconfiguration" y
	fi
done

print_row() {
	printf "| %-19.19s | %-30.30s | %-51.51s | %-50.50s | \n" "$1" "$2" "$3" "$4"
}

echo
echo "-------------------------------------------------------------------------------------------------------------------------------------------------------------------"
echo "| The following non-default values have not been converted:                                                                                                       |"
echo "|-----------------------------------------------------------------------------------------------------------------------------------------------------------------|"
echo "| Resource            | Key                            | Recommended Action                                  | Value                                              |"
echo "|---------------------|--------------------------------|-----------------------------------------------------|----------------------------------------------------|"

[[ "$(jq -r '.items[0].spec.controllerImage' "$TEMPDIR/linstorcontrollers.json")" == quay.io/piraeusdatastore/piraeus-server:* ]] || print_row LinstorCluster .spec.controllerImage "Adjust image configuration" "$(jq -r '.items[0].spec.controllerImage' "$TEMPDIR/linstorcontrollers.json")"
[ -z "$(jq -r '.items[0].spec.dbCertSecret // ""' "$TEMPDIR/linstorcontrollers.json")" ] || print_row LinstorController .spec.dbCertSecret "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.dbCertSecret' "$TEMPDIR/linstorcontrollers.json")"
[ "$(jq -r '.items[0].spec.dbUseClientCert' "$TEMPDIR/linstorcontrollers.json")" == "false" ] || print_row LinstorController .spec.dbUseClientCert "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.dbUseClientCert' "$TEMPDIR/linstorcontrollers.json")"
[ -z "$(jq -r '.items[0].spec.drbdRepoCred' "$TEMPDIR/linstorcontrollers.json")" ] || print_row LinstorController .spec.drbdRepoCred "Use pull secret to deploy the operator" "$(jq -r '.items[0].spec.drbdRepoCred' "$TEMPDIR/linstorcontrollers.json")"
[ -z "$(jq -r '.items[0].spec.httpBindAddress // ""' "$TEMPDIR/linstorcontrollers.json")" ] || print_row LinstorController .spec.httpBindAddress "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.httpBindAddress' "$TEMPDIR/linstorcontrollers.json")"
[ -z "$(jq -r '.items[0].spec.httpsBindAddress // ""' "$TEMPDIR/linstorcontrollers.json")" ] || print_row LinstorController .spec.httpsBindAddress "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.httpsBindAddress' "$TEMPDIR/linstorcontrollers.json")"
[ "$(jq -r '.items[0].spec.imagePullPolicy' "$TEMPDIR/linstorcontrollers.json")" == "IfNotPresent" ] || print_row LinstorController .spec.imagePullPolicy "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.imagePullPolicy' "$TEMPDIR/linstorcontrollers.json")"
[ "$(jq -r '.items[0].spec.logLevel // "info"' "$TEMPDIR/linstorcontrollers.json")" == "info" ] || print_row LinstorController .spec.logLevel "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.logLevel' "$TEMPDIR/linstorcontrollers.json")"
[ -z "$(jq -r '.items[0].spec.priorityClassName' "$TEMPDIR/linstorcontrollers.json")" ] || print_row LinstorController .spec.priorityClassName "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.priorityClassName' "$TEMPDIR/linstorcontrollers.json")"
[ "$(jq '.items[0].spec.replicas' "$TEMPDIR/linstorcontrollers.json")" -eq 1 ] || print_row LinstorController .spec.replicas "Keep at one" "$(jq '.items[0].spec.replicas' "$TEMPDIR/linstorcontrollers.json")"
[ "$(jq '.items[0].spec.resources | length' "$TEMPDIR/linstorcontrollers.json")" -eq 0 ] || print_row LinstorController .spec.resources "Use LinstorCluster.spec.patches" "$(jq -c '.items[0].spec.resources' "$TEMPDIR/linstorcontrollers.json")"
[ -z "$(jq -r '.items[0].spec.serviceAccountName // ""' "$TEMPDIR/linstorcontrollers.json")" ] || print_row LinstorController .spec.serviceAccountName "Let Operator manage RBAC" "$(jq -r '.items[0].spec.serviceAccountName' "$TEMPDIR/linstorcontrollers.json")"

[ "$(jq -r '.items[0].spec.controllerEndpoint' "$TEMPDIR/linstorcsidrivers.json")" == "$EXPECTED_CONTROLLER_ENDPOINT" ] || print_row LinstorCSIDriver .spec.controllerEndpoint "Set LinstorCluster.spec.externalController" "$(jq -r '.items[0].spec.controllerEndpoint' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.csiAttacherImage' "$TEMPDIR/linstorcsidrivers.json")" == registry.k8s.io/sig-storage/csi-attacher:* ]] || print_row LinstorCSIDriver .spec.csiAttacherImage "Adjust image configuration" "$(jq -r '.items[0].spec.csiAttacherImage' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.csiLivenessProbeImage' "$TEMPDIR/linstorcsidrivers.json")" == registry.k8s.io/sig-storage/livenessprobe:* ]] || print_row LinstorCSIDriver .spec.csiLivenessProbeImage "Adjust image configuration" "$(jq -r '.items[0].spec.csiLivenessProbeImage' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.csiNodeDriverRegistrarImage' "$TEMPDIR/linstorcsidrivers.json")" == registry.k8s.io/sig-storage/csi-node-driver-registrar:* ]] || print_row LinstorCSIDriver .spec.csiNodeDriverRegistrarImage "Adjust image configuration" "$(jq -r '.items[0].spec.csiNodeDriverRegistrarImage' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.csiProvisionerImage' "$TEMPDIR/linstorcsidrivers.json")" == registry.k8s.io/sig-storage/csi-provisioner:* ]] || print_row LinstorCSIDriver .spec.csiProvisionerImage "Adjust image configuration" "$(jq -r '.items[0].spec.csiProvisionerImage' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.csiResizerImage' "$TEMPDIR/linstorcsidrivers.json")" == registry.k8s.io/sig-storage/csi-resizer:* ]] || print_row LinstorCSIDriver .spec.csiResizerImage "Adjust image configuration" "$(jq -r '.items[0].spec.csiResizerImage' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.csiSnapshotterImage' "$TEMPDIR/linstorcsidrivers.json")" == registry.k8s.io/sig-storage/csi-snapshotter:* ]] || print_row LinstorCSIDriver .spec.csiSnapshotterImage "Adjust image configuration" "$(jq -r '.items[0].spec.csiSnapshotterImage' "$TEMPDIR/linstorcsidrivers.json")"
[[ "$(jq -r '.items[0].spec.linstorPluginImage' "$TEMPDIR/linstorcsidrivers.json")" == quay.io/piraeusdatastore/piraeus-csi:* ]] || print_row LinstorCSIDriver .spec.linstorPluginImage "Adjust image configuration" "$(jq -r '.items[0].spec.linstorPluginImage' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq '.items[0].spec.resources | length' "$TEMPDIR/linstorcsidrivers.json")" -eq 0 ] || print_row LinstorCSIDriver .spec.resources "Use LinstorCluster.spec.patches" "$(jq -c '.items[0].spec.resources' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq -r '.items[0].spec.csiControllerServiceAccountName' "$TEMPDIR/linstorcsidrivers.json")" == "csi-controller" ] || print_row LinstorCSIDriver .spec.csiControllerServiceAccountName "Let Operator manage RBAC" "$(jq -r '.items[0].spec.csiControllerServiceAccountName' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq -r '.items[0].spec.csiNodeServiceAccountName' "$TEMPDIR/linstorcsidrivers.json")" == "csi-node" ] || print_row LinstorCSIDriver .spec.csiNodeServiceAccountName "Let Operator manage RBAC" "$(jq -r '.items[0].spec.csiNodeServiceAccountName' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq '.items[0].spec.csiAttacherWorkerThreads // 10' "$TEMPDIR/linstorcsidrivers.json")" -eq 10 ] || print_row LinstorCSIDriver .spec.csiAttacherWorkerThreads "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.csiAttacherWorkerThreads' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq '.items[0].spec.csiProvisionerWorkerThreads // 10' "$TEMPDIR/linstorcsidrivers.json")" -eq 10 ] || print_row LinstorCSIDriver .spec.csiProvisionerWorkerThreads "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.csiProvisionerWorkerThreads' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq '.items[0].spec.csiResizerWorkerThreads // 10' "$TEMPDIR/linstorcsidrivers.json")" -eq 10 ] || print_row LinstorCSIDriver .spec.csiResizerWorkerThreads "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.csiResizerWorkerThreads' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq '.items[0].spec.csiSnapshotterWorkerThreads // 10' "$TEMPDIR/linstorcsidrivers.json")" -eq 10 ] || print_row LinstorCSIDriver .spec.csiSnapshotterWorkerThreads "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.csiSnapshotterWorkerThreads' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq '.items[0].spec.enableTopology' "$TEMPDIR/linstorcsidrivers.json")" == "true" ] || print_row LinstorCSIDriver .spec.enableTopology "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.enableTopology' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq -r '.items[0].spec.imagePullPolicy' "$TEMPDIR/linstorcsidrivers.json")" == "IfNotPresent" ] || print_row LinstorCSIDriver .spec.imagePullPolicy "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.imagePullPolicy' "$TEMPDIR/linstorcsidrivers.json")"
[ -z "$(jq -r '.items[0].spec.imagePullSecret' "$TEMPDIR/linstorcsidrivers.json")" ] || print_row LinstorCSIDriver .spec.imagePullSecret "Use pull secret to deploy the operator" "$(jq -r '.items[0].spec.imagePullSecret' "$TEMPDIR/linstorcsidrivers.json")"
[ "$(jq -r '.items[0].spec.logLevel // "info"' "$TEMPDIR/linstorcsidrivers.json")" == "info" ] || print_row LinstorCSIDriver .spec.logLevel "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.logLevel' "$TEMPDIR/linstorcsidrivers.json")"
[ -z "$(jq -r '.items[0].spec.priorityClassName' "$TEMPDIR/linstorcsidrivers.json")" ] || print_row LinstorCSIDriver .spec.priorityClassName "Use LinstorCluster.spec.patches" "$(jq -r '.items[0].spec.priorityClassName' "$TEMPDIR/linstorcsidrivers.json")"

for IDX in $(seq 0 "$(jq '.items | length - 1' "$TEMPDIR/linstorsatellitesets.json")"); do
	[ "$(jq_item '.spec.affinity?.nodeAffinity?.preferredDuringSchedulingIgnoredDuringExecution | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution "Use LinstorSatelliteConfiguration.spec.nodeAffinity" "$(jq_item -c '.spec.affinity.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution')"
	[ "$(jq_item '.spec.affinity?.podAffinity | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.affinity.podAffinity "Use LinstorSatelliteConfiguration.spec.nodeAffinity" "$(jq_item -c '.spec.affinity.podAffinity')"
	[ "$(jq_item '.spec.affinity?.podAntiAffinity | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.affinity.podAntiAffinity "Use LinstorSatelliteConfiguration.spec.nodeAffinity" "$(jq_item -c '.spec.affinity.podAntiAffinity')"
	[ "$(jq_item '.spec.storagePools.zfsPools | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.storagePools.zfsPools "Manually configure ZFS pools" "$(jq_item -c '.spec.storagePools.zfsPools')"
	[ "$(jq_item -r '.spec.automaticStorageType')" == "None" ] || print_row LinstorSatelliteSet .spec.automaticStorageType "Use LinstorSatelliteConfiguration.spec.storagePools" "$(jq_item -r '.spec.automaticStorageType')"
	[ "$(jq_item -r '.spec.controllerEndpoint')" == "$EXPECTED_CONTROLLER_ENDPOINT" ] || print_row LinstorSatelliteSet .spec.controllerEndpoint "Set LinstorCluster.spec.externalController" "$(jq_item -r '.spec.controllerEndpoint')"
	[ -z "$(jq_item -r '.spec.drbdRepoCred')" ] || print_row LinstorSatelliteSet .spec.drbdRepoCred "Use pull secret to deploy the operator" "$(jq_item -r '.spec.drbdRepoCred')"
	[ "$(jq_item -r '.spec.imagePullPolicy')" == "IfNotPresent" ] || print_row LinstorSatelliteSet .spec.imagePullPolicy "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -r '.spec.imagePullPolicy')"
	[ -z "$(jq_item -r '.spec.kernelModuleInjectionAdditionalSourceDirectory // ""')" ] || print_row LinstorSatelliteSet .spec.kernelModuleInjectionAdditionalSourceDirectory "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -r '.spec.kernelModuleInjectionAdditionalSourceDirectory')"
	[ "$(jq_item -r '.spec.kernelModuleInjectionExtraVolumeMounts | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.kernelModuleInjectionExtraVolumeMounts "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -c '.spec.kernelModuleInjectionExtraVolumeMounts')"
	[[ "$(jq_item -r '.spec.kernelModuleInjectionImage')" == quay.io/piraeusdatastore/drbd9-* ]] || print_row LinstorSatelliteSet .spec.kernelModuleInjectionImage "Adjust image configuration" "$(jq_item -r '.spec.kernelModuleInjectionImage')"
	[ "$(jq_item -r '.spec.kernelModuleInjectionMode')" == "Compile" ] || print_row LinstorSatelliteSet .spec.kernelModuleInjectionMode "Adjust image configuration" "$(jq_item -r '.spec.kernelModuleInjectionMode')"
	[ "$(jq_item -r '.spec.kernelModuleInjectionResources | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.kernelModuleInjectionResources "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -c '.spec.kernelModuleInjectionResources')"
	[ "$(jq_item -r '.spec.logLevel // "info"')" == "info" ] || print_row LinstorSatelliteSet .spec.logLevel "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -r '.spec.logLevel')"
	[ -z "$(jq_item -r '.spec.monitoringBindAddress // ""')" ] || print_row LinstorSatelliteSet .spec.monitoringBindAddress "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -r '.spec.monitoringBindAddress')"
	[[ "$(jq_item -r '.spec.monitoringImage')" == quay.io/piraeusdatastore/drbd-reactor:* ]] || print_row LinstorSatelliteSet .spec.monitoringImage "Adjust image configuration" "$(jq_item -r '.spec.monitoringImage')"
	[ -z "$(jq_item -r '.spec.priorityClassName')" ] || print_row LinstorSatelliteSet .spec.priorityClassName "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -r '.spec.priorityClassName')"
	[[ "$(jq_item -r '.spec.satelliteImage')" == quay.io/piraeusdatastore/piraeus-server:* ]] || print_row LinstorSatelliteSet .spec.satelliteImage "Adjust image configuration" "$(jq_item -r '.spec.satelliteImage')"
	[ "$(jq_item -r '.spec.resources | length')" -eq 0 ] || print_row LinstorSatelliteSet .spec.resources "Use LinstorSatelliteConfiguration.spec.patches" "$(jq_item -c '.spec.resources')"
	[ -z "$(jq_item -r '.spec.serviceAccountName // ""')" ] || print_row LinstorSatelliteSet .spec.serviceAccountName "Let Operator manage RBAC" "$(jq_item -c '.spec.serviceAccountName')"
done

echo "-------------------------------------------------------------------------------------------------------------------------------------------------------------------"

echo
echo "---------------------------------------------------------------------------------------------"
echo "| After upgrading, apply the following resources. They have been saved to v2-resources.yaml |"
echo "---------------------------------------------------------------------------------------------"
echo "---"
kubectl kustomize "$TEMPDIR" | tee v2-resources.yaml
