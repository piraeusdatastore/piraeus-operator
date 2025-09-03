#!/bin/sh
set -e

YQ="${YQ:-yq}"
KUSTOMIZE="${KUSTOMIZE:-kustomize}"

{
	cat <<EOF
# DO NOT EDIT; Automatically created by hack/copy-rbac-config-to-chart.sh
{{ if .Values.rbac.create }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "piraeus-operator.fullname" . }}-controller-manager
  labels:
    {{- include "piraeus-operator.labels" . | nindent 4 }}
rules:
EOF

	${KUSTOMIZE} build config/default | ${YQ} eval 'select(.kind=="ClusterRole" and .metadata.name=="piraeus-operator-controller-manager").rules'

	cat <<EOF
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "piraeus-operator.fullname" . }}-leader-election
  labels:
    {{- include "piraeus-operator.labels" . | nindent 4 }}
rules:
EOF

	${KUSTOMIZE} build config/default | ${YQ} eval 'select(.kind=="Role" and .metadata.name=="piraeus-operator-leader-election-role").rules'

	cat <<EOF >> charts/piraeus/templates/rbac.yaml
{{ end }}
EOF

}> charts/piraeus/templates/rbac.yaml
