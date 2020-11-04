{{- define "post-install-labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service | quote }}
app.kubernetes.io/instance: {{ .Release.Name | quote }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
helm.sh/chart: "{{ .Chart.Name }}-{{ .Chart.Version }}"
{{- end -}}
{{- define "post-install-annotations" -}}
"helm.sh/hook": post-install,post-upgrade
"helm.sh/hook-delete-policy": hook-succeeded
{{- end -}}
