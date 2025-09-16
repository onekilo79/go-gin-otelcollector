## 0. Expected tooling to run this project

1. Go
2. Docker & Docker-Compose 

This version in docker-compose we only start Jaeger, OpenTelemetry, Prometheus.

You start the server from another command prompt or within your IDE. 


## 1. Build the docker image

```bash
make docker-build-album
```

## 2. Start All Observability & Log Viewing Services
 
```bash
make docker-compose-limited-start;
```

## 3. Start album-service Go/Gin Server

### 3.1 From the Command line 

```bash
make local-start;
```

### 3.2 From your IDE

#### 3.2.1 Visual Studio Code

Debug the application from the provided launch.json

#### 3.2.2 Goland

You will need to set your Environment with the following and run the main.go

`GRPC_GO_LOG_SEVERITY_LEVEL=info;GRPC_GO_LOG_VERBOSITY_LEVEL=99;INSTANCE_NAME=album-service-1;NAMESPACE=no-namespace;OTEL_LOCATION=localhost:4327`

#### Note: the application will not start without the OpenTelemetry collector running

## 4. Run Some Tests

### 4.1 from the command line

```bash
make local-test;
```

### 4.2 Using Postman

[Postman files](../test/.)

1. Import the collection and environment into your postman
1. Set Environment to `localhost`
1. Open a test in the `album-service` collection and run it.

## 5. View the events in the different Services

[View Jaeger](http://localhost:16696/search?limit=20&service=album-service)

[View Prometheus](http://localhost:9090/graph?g0.expr=%7Bjob%3D~%22.%2B%22%7D%20&g0.tab=0&g0.stacked=0&g0.show_exemplars=0&g0.range_input=1h)

## 6. Stop album-service server & Services  

### 1. Stop Server

`Ctr + C` in the terminal window where go is running. 

### 2. Stop Observability and Log Viewing Services

```bash
make docker-compose-limited-stop;
```