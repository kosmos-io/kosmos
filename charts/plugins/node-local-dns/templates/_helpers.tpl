{{- define "node-local-dns.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "node-local-dns.namespace" -}}
{{- default .Chart.Namespace .Values.namespaceOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
