# Copilot Instructions for go-gin-otelcollector

## Project Overview
- This is a Go backend for a music store, using the Gin framework and OpenTelemetry for observability.
- Two main services: `album-store` (core API) and `proxy-service` (proxies requests to album-store, demonstrates nested OpenTelemetry spans).
- Designed for deployment in Kubernetes (K3D, Microk8s, etc.) and Docker Compose. Helm charts are provided for all major components.

## Architecture & Data Flow
- `album-store` exposes REST endpoints and is instrumented with OpenTelemetry.
- `proxy-service` uses `otelhttp.client` to call `album-store`, producing nested spans for tracing.
- Observability stack includes Jaeger, Prometheus, and OpenTelemetry Collector.
- Istio is used for ingress and service mesh features (see `install/helm/istio-ingress-charts`).

## Developer Workflows
- **Build & Test:**
  - Run `make && make local-proxy-test` to build and test locally.
- **Run Locally:**
  - Use Docker Compose: see `docs/Run-Docker-Compose-Install-Full.md` and `docs/Run-Docker-Compose-Install-Limited.md`.
  - For Kubernetes: see `docs/K3D-run.md` and related K3D/Microk8s docs.
- **Helm Charts:**
  - Charts for all services in `install/helm/`.
  - To update charts: run the sequence in `install/helm/README.md` (see example commands for packaging and indexing charts).

## Conventions & Patterns
- Environment variables for service URLs and OpenTelemetry collector location are set in Helm values (see `deployment.env` in chart READMEs).
- Istio sidecar injection is enabled by default for services (`sidecar.istio.io/inject: true`).
- Images are pulled from a local registry (`registry.local:54094`).
- All services default to HTTP, with TLS disabled unless overridden in Helm values.
- Dashboards for Grafana are managed via configmaps and toggled in Helm values.

## Integration Points
- External dependencies: Jaeger, Prometheus, OpenTelemetry Collector, Grafana.
- Service communication: `proxy-service` calls `album-store` via HTTP, with tracing enabled.
- Helm repo can be added via: `helm repo add go-gin-opentelemetry 'https://mcarr-and.github.io/go-gin-otelcollector/install/helm/charts'`

## Key Files & Directories
- `main.go`, `main_test.go`: entry points for both services.
- `install/helm/`: Helm charts and repo management.
- `docs/`: step-by-step guides for running locally and in Kubernetes.
- `model/`: shared Go models.
- `proxy/`: proxy-service implementation.
- `api/`: OpenAPI/Swagger docs for both services.

## Example: Nested Spans
- To view nested spans, run the proxy-service and inspect Jaeger at [http://localhost:16696/](http://localhost:16696/). Select `proxy-service` in the Jaeger UI.

---

For unclear or missing conventions, please ask for clarification or point to specific files for deeper analysis.
