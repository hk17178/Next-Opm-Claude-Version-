{{/*
OpsNexus Helm Chart - Template Helpers
*/}}

{{/*
Expand the name of the chart.
*/}}
{{- define "opsnexus.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "opsnexus.fullname" -}}
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
Chart label
*/}}
{{- define "opsnexus.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "opsnexus.labels" -}}
helm.sh/chart: {{ include "opsnexus.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: opsnexus
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}

{{/*
Service-specific labels
*/}}
{{- define "opsnexus.svcLabels" -}}
{{ include "opsnexus.labels" .context }}
app.kubernetes.io/name: {{ .svcName }}
app.kubernetes.io/instance: {{ .context.Release.Name }}
app.kubernetes.io/component: {{ .svcName }}
{{- end }}

{{/*
Service-specific selector labels
*/}}
{{- define "opsnexus.svcSelectorLabels" -}}
app.kubernetes.io/name: {{ .svcName }}
app.kubernetes.io/instance: {{ .context.Release.Name }}
{{- end }}

{{/*
Get image for a service.
Usage: {{ include "opsnexus.image" (dict "svc" .Values.svc-log "global" .Values.global "svcName" "svc-log") }}
*/}}
{{- define "opsnexus.image" -}}
{{- $registry := .global.image.registry -}}
{{- $repo := .svc.image.repository -}}
{{- $tag := default .global.image.tag .svc.image.tag -}}
{{- printf "%s/%s:%s" $registry $repo $tag -}}
{{- end }}

{{/*
Standard deployment template for a backend service.
Accepts a dict with keys: svcName, svcValues, context (root context)
*/}}
{{- define "opsnexus.deployment" -}}
{{- $svcName := .svcName -}}
{{- $svc := .svcValues -}}
{{- $ctx := .context -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ $svcName }}
  namespace: {{ $ctx.Release.Namespace }}
  labels:
    {{- include "opsnexus.svcLabels" (dict "svcName" $svcName "context" $ctx) | nindent 4 }}
spec:
  replicas: {{ $svc.replicas }}
  selector:
    matchLabels:
      {{- include "opsnexus.svcSelectorLabels" (dict "svcName" $svcName "context" $ctx) | nindent 6 }}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        {{- include "opsnexus.svcLabels" (dict "svcName" $svcName "context" $ctx) | nindent 8 }}
    spec:
      {{- with $ctx.Values.global.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: {{ $svcName }}
          image: {{ include "opsnexus.image" (dict "svc" $svc "global" $ctx.Values.global "svcName" $svcName) }}
          imagePullPolicy: {{ $ctx.Values.global.image.pullPolicy }}
          ports:
            - name: http
              containerPort: {{ $svc.service.httpPort }}
              protocol: TCP
            - name: grpc
              containerPort: {{ $svc.service.grpcPort }}
              protocol: TCP
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 15
            periodSeconds: 20
            timeoutSeconds: 5
            failureThreshold: 3
          resources:
            {{- toYaml $svc.resources | nindent 12 }}
          envFrom:
            - configMapRef:
                name: opsnexus-config
            - secretRef:
                name: opsnexus-secret
          env:
            - name: SERVICE_NAME
              value: {{ $svcName }}
            - name: OPSNEXUS_DB_NAME
              value: {{ index $ctx.Values.database.databases $svcName }}
            - name: OPSNEXUS_DB_HOST
              valueFrom:
                secretKeyRef:
                  name: opsnexus-secret
                  key: db-host
            - name: OPSNEXUS_DB_PORT
              valueFrom:
                secretKeyRef:
                  name: opsnexus-secret
                  key: db-port
            - name: OPSNEXUS_DB_USER
              valueFrom:
                secretKeyRef:
                  name: opsnexus-secret
                  key: db-user
            - name: OPSNEXUS_DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: opsnexus-secret
                  key: db-password
      {{- if $ctx.Values.podAntiAffinity.enabled }}
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    {{- include "opsnexus.svcSelectorLabels" (dict "svcName" $svcName "context" $ctx) | nindent 20 }}
                topologyKey: kubernetes.io/hostname
      {{- end }}
{{- end }}

{{/*
Standard service template for a backend service.
*/}}
{{- define "opsnexus.service" -}}
{{- $svcName := .svcName -}}
{{- $svc := .svcValues -}}
{{- $ctx := .context -}}
apiVersion: v1
kind: Service
metadata:
  name: {{ $svcName }}
  namespace: {{ $ctx.Release.Namespace }}
  labels:
    {{- include "opsnexus.svcLabels" (dict "svcName" $svcName "context" $ctx) | nindent 4 }}
spec:
  type: ClusterIP
  selector:
    {{- include "opsnexus.svcSelectorLabels" (dict "svcName" $svcName "context" $ctx) | nindent 4 }}
  ports:
    - name: http
      port: {{ $svc.service.httpPort }}
      targetPort: http
      protocol: TCP
    - name: grpc
      port: {{ $svc.service.grpcPort }}
      targetPort: grpc
      protocol: TCP
{{- end }}

{{/*
Standard HPA template for a backend service.
*/}}
{{- define "opsnexus.hpa" -}}
{{- $svcName := .svcName -}}
{{- $svc := .svcValues -}}
{{- $ctx := .context -}}
{{- if $svc.autoscaling.enabled }}
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {{ $svcName }}
  namespace: {{ $ctx.Release.Namespace }}
  labels:
    {{- include "opsnexus.svcLabels" (dict "svcName" $svcName "context" $ctx) | nindent 4 }}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {{ $svcName }}
  minReplicas: {{ $svc.autoscaling.minReplicas }}
  maxReplicas: {{ $svc.autoscaling.maxReplicas }}
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: {{ $svc.autoscaling.targetCPUUtilization }}
{{- end }}
{{- end }}
