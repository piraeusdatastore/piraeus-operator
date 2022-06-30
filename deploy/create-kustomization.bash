#! /bin/bash

set -e

dir="deploy/"
kustomize="kustomization.yaml"

pushd $dir

cat > "$kustomize" <<EOL
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: piraeus-operator
resources:
EOL


files="$(find -L "piraeus" -type f | sort)"
echo "$files" | while read -r file; do
  echo "  - $file" >> "$kustomize"
done

popd
