{{/*
Expand the name of the chart.
*/}}
{{- define "runtz.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "runtz.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "runtz.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "runtz.labels" -}}
helm.sh/chart: {{ include "runtz.chart" . }}
app.kubernetes.io/name: {{ include "runtz.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Component resource name.
*/}}
{{- define "runtz.componentName" -}}
{{- $root := index . "root" -}}
{{- $component := index . "component" -}}
{{- printf "%s-%s" (include "runtz.fullname" $root) $component | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Component selector labels.
*/}}
{{- define "runtz.componentSelectorLabels" -}}
{{- $root := index . "root" -}}
{{- $component := index . "component" -}}
app.kubernetes.io/name: {{ include "runtz.name" $root }}
app.kubernetes.io/instance: {{ $root.Release.Name }}
app.kubernetes.io/component: {{ $component }}
{{- end }}

{{/*
Component labels.
*/}}
{{- define "runtz.componentLabels" -}}
{{- $root := index . "root" -}}
helm.sh/chart: {{ include "runtz.chart" $root }}
{{ include "runtz.componentSelectorLabels" . }}
{{- if $root.Chart.AppVersion }}
app.kubernetes.io/version: {{ $root.Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ $root.Release.Service }}
{{- end }}

{{- define "runtz.frontendName" -}}
{{- include "runtz.componentName" (dict "root" . "component" "frontend") }}
{{- end }}

{{- define "runtz.backendName" -}}
{{- include "runtz.componentName" (dict "root" . "component" "backend") }}
{{- end }}

{{- define "runtz.frontendConfigMapName" -}}
{{- printf "%s-config" (include "runtz.frontendName" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "runtz.backendConfigMapName" -}}
{{- printf "%s-config" (include "runtz.backendName" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "runtz.mongodbName" -}}
{{- include "runtz.componentName" (dict "root" . "component" "mongodb") }}
{{- end }}

{{- define "runtz.backendSecretName" -}}
{{- default (include "runtz.backendName" .) .Values.backend.secrets.existingSecret }}
{{- end }}

{{/*
Create the name of the service account to use.
*/}}
{{- define "runtz.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "runtz.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
