{{/* Adapted from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6. */}}
{{- define "signal-foundry.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "signal-foundry.fullname" -}}
{{- if .Values.fullnameOverride }}{{ .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}{{ else }}{{ printf "%s-%s" .Release.Name (include "signal-foundry.name" .) | trunc 63 | trimSuffix "-" }}{{ end }}
{{- end }}
{{- define "signal-foundry.labels" -}}
app.kubernetes.io/name: {{ include "signal-foundry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end }}
{{- define "signal-foundry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "signal-foundry.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- define "signal-foundry.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}{{ default (include "signal-foundry.fullname" .) .Values.serviceAccount.name }}{{ else }}{{ default "default" .Values.serviceAccount.name }}{{ end }}
{{- end }}
{{- define "signal-foundry.image" -}}
{{- if .Values.image.digest }}{{ printf "%s@%s" .Values.image.repository .Values.image.digest }}{{ else }}{{ printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}{{ end }}
{{- end }}
{{- define "signal-foundry.envFrom" -}}
{{- if .Values.existingConfigMap }}
- configMapRef: {name: {{ .Values.existingConfigMap }}}
{{- end }}
{{- if .Values.existingSecret }}
- secretRef: {name: {{ .Values.existingSecret }}}
{{- end }}
{{- end }}
