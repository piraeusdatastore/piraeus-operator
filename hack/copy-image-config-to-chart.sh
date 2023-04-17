#!/bin/sh

cat <<EOF > charts/piraeus/templates/config.yaml
# DO NOT EDIT; Automatically created by hack/copy-image-config-to-chart.sh
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "piraeus-operator.fullname" . }}-image-config
  labels:
  {{- include "piraeus-operator.labels" . | nindent 4 }}
data:
  images.yaml: |
EOF

sed 's/^/    /' config/manager/images.yaml >> charts/piraeus/templates/config.yaml
