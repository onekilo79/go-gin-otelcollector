### Tools used

1. Go
2. Docker
3. [Skaffold](https://skaffold.dev/)
4. [K3D](https://k3d.io/v5.4.6/)
5. [homebrew](https://brew.sh/)

Extra Documentation for K3D.

1. [K3d Registry info](K3D-registry.md)
2. [K3d Debugging commands for cluster](K3D-Debugging.md)

## 0 Expected installs

## 0.1 Install tools need for Apple developers

```bash
    brew install skaffold helm go kubernetes-cli docker k3d;
```

### 0.2 /etc/hosts

local changes to your `/etc/hosts` to use nginx-ingress with the k3d cluster.

```127.0.0.1	localhost k-dashboard.local jaeger.local otel-collector.local grafana.local prometheus.local kiali.local album-service.local proxy-serivce.local```

## 1. Create K3d Kubernetes Cluster with Internal Registry

```bash
  make k3d-cluster-create;
```

## 2. Build the application in Docker and push Docker image to the  K3D Internal Registry

```bash
  make docker-tag-k3d-registry-album;
  cd proxy;
  make docker-tag-k3d-registry-proxy;
  cd ..;
```

## 3. Install to cluster: Applications, Observability tooling, Monitoring tooling.

```bash
  export ISTIO_GATEWAY_EXTERNAL_IP=localhost;
  make skaffold-k3d-dev;
```

**Note:**

The album-service will not start after printing its version number if OpenTelemetry-collector cannot be reached.

This will mean the liveness probe will fail and the album-service will eventually be in a CrashLoopBackoff state when you get pods.


## 4. Run Some Tests

### 4.1 curl

```bash
curl --insecure --location 'http://album-service.local:8070/albums/'; 
```

### 4.3 Run Test Suite

```bash
make k3d-test;
```

### 4.2 Postman

[Postman files](../test/.)

1. Import the folder `../test`
1. Set Environment to `album-service.local`
1. Open a test in the `album-service` collection and run it.

## 5. View the events in the different Services in K3D`

Services:
* [Jaeger to see Observability spans](http://jaeger.local:8070/search?limit=20&service=album-service)
* [Prometheus for metrics](http://prometheus.local:8070)
* [Grafana for dashboards](http://grafana.local:8070)
* [Kubernetes dashboard for visualising the cluster](http://k-dashboard.local:8070)
* [Kiali to visualise Istio](http://kiali.local:8070)
  * generate token `kubectl -n istio-system create token kiali;`

Applications:
* [Album Store](http://album-service.local:8070)
* [Proxy Service](http://proxy-service.local:8070)

## 6. Uninstall from cluster: Applications, Observability tooling, Monitoring tooling.  

Ctr + C on the terminal window where you started `make skaffold-k3d-dev`

## 7. Delete Cluster

```bash
  make k3d-cluster-delete;
```

