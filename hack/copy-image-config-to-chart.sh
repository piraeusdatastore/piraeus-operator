#!/bin/sh
set -e

cat <<EOF > charts/piraeus/templates/config.yaml
# DO NOT EDIT; Automatically created by hack/copy-image-config-to-chart.sh
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "piraeus-operator.fullname" . }}-image-config
  labels:
  {{- include "piraeus-operator.labels" . | nindent 4 }}
data:
  0_piraeus_datastore_images.yaml: |
EOF

sed 's/^/    /' config/manager/0_piraeus_datastore_images.yaml >> charts/piraeus/templates/config.yaml
cat <<EOF >> charts/piraeus/templates/config.yaml
  0_sig_storage_images.yaml: |
EOF
sed 's/^/    /' config/manager/0_sig_storage_images.yaml >> charts/piraeus/templates/config.yaml
