{{/* Adapted from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6. */}}
{{- define "sumweave.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- define "sumweave.fullname" -}}
{{- if .Values.fullnameOverride }}{{ .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}{{ else }}{{ printf "%s-%s" .Release.Name (include "sumweave.name" .) | trunc 63 | trimSuffix "-" }}{{ end }}
{{- end }}
{{- define "sumweave.labels" -}}
app.kubernetes.io/name: {{ include "sumweave.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end }}
{{- define "sumweave.selectorLabels" -}}
app.kubernetes.io/name: {{ include "sumweave.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- define "sumweave.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}{{ default (include "sumweave.fullname" .) .Values.serviceAccount.name }}{{ else }}{{ default "default" .Values.serviceAccount.name }}{{ end }}
{{- end }}
{{- define "sumweave.image" -}}
{{- if .Values.image.digest }}{{ printf "%s@%s" .Values.image.repository .Values.image.digest }}{{ else }}{{ printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) }}{{ end }}
{{- end }}
{{- define "sumweave.envFrom" -}}
{{- if .Values.existingConfigMap }}
- configMapRef: {name: {{ .Values.existingConfigMap }}}
{{- end }}
{{- if .Values.existingSecret }}
- secretRef: {name: {{ .Values.existingSecret }}}
{{- end }}
{{- end }}
