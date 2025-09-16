## 0. Expected tooling to run this project

1. Go
2. Docker & Docker-Compose


This version everything is running inside the docker-compose.


## 1. Start All Observability & Log Viewing Services
 
```bash
  make docker-compose-full-start;
```

#### Note: the application will not start without the OpenTelemetry collector running

## 2. Run Some Tests

### 2.1 from the command line

```bash
  make local-proxy-test;
```

### 2.2 Using Postman

[Postman files](../test/.)

1. Import the collection and environment into your postman
1. Set Environment to `localhost`
1. Open a test in the `album-service` collection and run it.

## 3. View the events in the different Services

[View Jaeger](http://localhost:16696/search?limit=20&service=album-service)

[View Prometheus](http://localhost:9090/graph?g0.expr=%7Bjob%3D~%22.%2B%22%7D%20&g0.tab=0&g0.stacked=0&g0.show_exemplars=0&g0.range_input=1h)

## 4. Stop everything  

```bash
  make docker-compose-full-stop;
```