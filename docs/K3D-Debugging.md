


# Ingress-Nginx

## Ingress-Nginx Logs(follow)

```bash
kubectl logs -f -n ingress-nginx $(kubectl -n ingress-nginx get pods -l app.kubernetes.io/name=ingress-nginx -o jsonpath="{.items[0].metadata.name}")
```
## Get Nginx configuration

```bash
kubectl exec -it -n ingress-nginx $(kubectl -n ingress-nginx get pods -l app.kubernetes.io/name=ingress-nginx -o jsonpath="{.items[0].metadata.name}") -- cat /etc/nginx/nginx.conf > nginx.conf
```

## Official documentation
https://kubernetes.github.io/ingress-nginx/troubleshooting/


# Album Store 

## album-service Logs for the 1st found pod(follow) 

```bash
kubectl logs -f -n album-service $(kubectl -n album-service get pods -l app.kubernetes.io/name=album-service -o jsonpath="{.items[0].metadata.name}")
```

# Test album-service with curl

```bash
curl --insecure --location 'http://album-service.local:8070/albums/'; 
```

# OpenTelemetry

## Opentelemetry-collector Logs(follow)

```bash
kubectl logs -f -n observability $(kubectl -n observability get pods  -l app.kubernetes.io/name=opentelemetry-collector -o jsonpath="{.items[0].metadata.name}")
```

# Get Prometheus admin password

```bash
kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode ; echo;
```