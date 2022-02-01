{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "operator.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "operator.labels" -}}
app.kubernetes.io/name: {{ include "operator.name" . }}
helm.sh/chart: {{ include "operator.chart" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Controller SSL secret name
*/}}
{{- define "controller.sslSecretName" -}}
  {{- if (eq (.Values.linstorSslMethod | default "manual") "manual") -}}
    {{- .Values.operator.controller.sslSecret }}
  {{- else }}
    {{- .Values.operator.controller.sslSecret | default (printf "%s-control-secret" ( include "operator.fullname" . )) }}
  {{- end -}}
{{- end -}}

{{/*
SatelliteSet SSL secret name
*/}}
{{- define "satelliteSet.sslSecretName" -}}
  {{- if (eq (.Values.linstorSslMethod | default "manual") "manual") -}}
    {{- .Values.operator.satelliteSet.sslSecret }}
  {{- else }}
    {{- .Values.operator.satelliteSet.sslSecret | default (printf "%s-node-secret" ( include "operator.fullname" . )) }}
  {{- end -}}
{{- end -}}

{{/*
Controller HTTPS secret name
*/}}
{{- define "controller.httpsSecretName" -}}
  {{- if (eq (.Values.linstorHttpsMethod | default "manual") "manual") -}}
    {{- .Values.linstorHttpsControllerSecret }}
  {{- else }}
    {{- .Values.linstorHttpsControllerSecret | default (printf "%s-controller-secret" ( include "operator.fullname" . )) }}
  {{- end -}}
{{- end -}}

{{/*
Client HTTPS secret name
*/}}
{{- define "client.httpsSecretName" -}}
  {{- if (eq (.Values.linstorHttpsMethod | default "manual") "manual") -}}
    {{- .Values.linstorHttpsClientSecret }}
  {{- else }}
    {{- .Values.linstorHttpsClientSecret | default (printf "%s-client-secret" ( include "operator.fullname" . )) }}
  {{- end -}}
{{- end -}}

{{/*
Endpoint URL of LINSTOR controller
*/}}
{{- define "controller.endpoint" -}}
  {{- if .Values.controllerEndpoint -}}
    {{ .Values.controllerEndpoint }}
  {{- else -}}
    {{- if empty ( include "client.httpsSecretName" . ) -}}
      http://{{ template "operator.fullname" . }}-cs.{{ .Release.Namespace }}.svc:3370
    {{- else -}}
      https://{{ template "operator.fullname" . }}-cs.{{ .Release.Namespace }}.svc:3371
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
Environment to pass to any containers using golinstor
*/}}
{{- define "linstor-env" -}}
- name: LS_CONTROLLERS
  value: {{ template "controller.endpoint" . }}
{{- if not (empty ( include "client.httpsSecretName" . )) }}
- name: LS_USER_CERTIFICATE
  valueFrom:
    secretKeyRef:
      name: {{ template "client.httpsSecretName" . }}
      key: tls.crt
- name: LS_USER_KEY
  valueFrom:
    secretKeyRef:
      name: {{ template "client.httpsSecretName" . }}
      key: tls.key
- name: LS_ROOT_CA
  valueFrom:
    secretKeyRef:
      name: {{ template "client.httpsSecretName" . }}
      key: ca.crt
{{- end -}}
{{- end -}}
{{/*
Sets the default security context for all pods
*/}}
{{- define "operator.defaultpodsecuritycontext" -}}
{{- if .Values.global.setSecurityContext -}}
runAsUser: 1000
runAsGroup: 1000
fsGroup: 1000
{{- end -}}
{{- end -}}
