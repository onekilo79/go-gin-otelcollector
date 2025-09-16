# Helm Repo for Project code.

## Helm Charts 

* Album Store
* Proxy Service
* Grafana Dashboards via Configmaps. Inspred by [Glenn De Haan's Grafana Helm](https://github.com/glenndehaan/charts/tree/master/charts/grafana-dashboards)
* Istio Ingress charts

## Basic info
Repository with k8s Helm repo.
Basically it's a repository with helm repo.
Check out this example:

- https://github.com/kmzfs/helm-repo-in-github

## Update Helm Repo

```bash
cd install/helm
helm-docs  
helm package album-service -d charts; 
helm package proxy-service -d charts;
helm package grafana-dashboards -d charts;
helm package istio-ingress-charts -d charts;
helm repo index ./charts; 
git add *;
git commit -m "helm chart updates"
git push
```

## Use Helm Repo
```bash
helm repo add go-gin-opentelemetry 'https://mcarr-and.github.io/go-gin-otelcollector/install/helm/charts'
helm repo update
helm repo list
```
