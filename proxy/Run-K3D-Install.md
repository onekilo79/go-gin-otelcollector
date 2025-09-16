### Tools used

1. Go
2. Docker
3. [Skaffold](https://skaffold.dev/)
4. [K3D](https://k3d.io/v5.4.6/)

## 0. Expected tooling to run this project in K3D

local changes to your `/etc/hosts` to use nginx-ingress with the k3d cluster.

`proxy-service.local` is included in this configuration if you are following the album-service config changes.

`127.0.0.1	localhost k-dashboard.local jaeger.local otel-collector.local grafana.local prometheus.local album-service.local proxy-service.local`

### 0.1 K3D Registry info

[K3d Registry info](../docs/K3D-registry.md)

## 1. Create K3d Kubernetes Cluster with Internal Registry

```bash
  make k3d-cluster-create;
```

## 2. Build the applications in Docker and push Docker image to the  K3D Internal Registry

```bash
  make docker-tag-k3d-registry-proxy && make docker-tag-k3d-registry-album;
```

## 3. Install to cluster: Applications, Observability tooling, Monitoring tooling.
 
```bash
  make skaffold-dev-k3d;
```

**Note:** 

The proxy-service will not start after printing its version number if OpenTelemetry-collector cannot be reached.

This will mean the liveness probe will fail and the proxy-service will eventually be in a CrashLoopBackoff state when you get pods.



### 3.1 Debugging Advice  

[Debugging commands for cluster](../docs/K3D-Debugging.md)

## 4. Run Some Tests

### 4.1 Command line

```bash
  curl --insecure --location 'http://proxy-service.local:8070/albums/'; 
```

```bash
  make k3d-test;
```

### 4.2 Postman

[Postman files](../test/.)

1. Import the folder `../test`
1. Set Environment to `proxy-service.local`
1. Open a test in the `album-service` collection and run it.

### 4.3 Browser

[view proxy-service albums](http://proxy-service.local:8070/albums)

## 5. View the events in the different Services in K3D

Each Span will also have 2 sub spans. 

* The original call to Proxy-Service.
  * 1 call http to album-service.
  * 1 call to album-service.

[View Jaeger to see spans](http://jaeger.local:8070/search?limit=20&service=proxy-service)

[View Kubernetes environment](http://k-dashboard.local:8070/)

[Grafana](http://grafana.local:8070/)

[Prometheus](http://prometheus.local:8070/)

## 6. Uninstall from cluster: Applications, Observability tooling, Monitoring tooling.  

Ctr + C on the terminal window where you started `make skaffold-dev`

## 7. Delete cluster

```bash
  make k3d-cluster-delete;
```