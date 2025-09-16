.PHONY: build
build: generate-swagger
	go mod tidy;
	go get;
	go clean;
	go build -ldflags "-X main.version=0.1 -X main.gitHash=`git rev-parse --short HEAD`" -v -o proxy-service-bin

.PHONY: test
test:
	go test -v -json > test-results.json

#.PHONY: test-benchmark
#test-benchmark:
#	go test -bench=. -count 2 -run=^# -benchmem

set-local-test:
	$(eval url_value := http://localhost:9070)

set-k3d-test:
	$(eval url_value := http://proxy-service.local:8070)

.PHONY: local-test
local-test: set-local-test run-tests

.PHONY: k3d-test
k3d-test: set-k3d-test run-tests

run-tests:
	curl --location --request GET '$(url_value)/albums/1' --header 'Accept: application/json';
	curl --location --request GET '$(url_value)/albums/666' --header 'Accept: application/json';
	curl --location --request GET '$(url_value)/albums/X' --header 'Accept: application/json';
	curl --location --request GET '$(url_value)/albums';
	curl --location --request POST '$(url_value)/albums' \
		--header 'Content-Type: application/json' --header 'Accept: application/json' \
		--data-raw '{ "idx": 10, "titlexx": "Blue Train", "artistx": "John Coltrane", "price": 56.99, "X": "asdf" }';
	curl --location --request POST '$(url_value)/albums' \
    		--header 'Content-Type: application/json' --header 'Accept: application/json' \
    		--data-raw '{ "id": -1, "title": "s", "artist": "p", "price": -0.1}';
	curl --location --request POST '$(url_value)/albums' \
        --header 'Content-Type: application/json' --header 'Accept: application/json' \
        --data-raw '{"id": 10,';
	curl --location --request POST '$(url_value)/albums' \
        --header 'Content-Type: application/json' --header 'Accept: application/json' \
        --data-raw '{"id": 10, "title": "The Ozzman Cometh", "artist": "Black Sabbath", "price": 66.60}';
	curl --location --request GET '$(url_value)/status';
	curl -s -S --output /dev/null --location --request GET '$(url_value)/metrics';

.PHONY: eval-git-hash
eval-git-hash:
	$(eval GIT_HASH:= $(shell git rev-parse --short HEAD))
	echo $(GIT_HASH)

.PHONY: build-raspberry-pi
build-raspberry-pi:
	$(eval BUILD_PLATFORM_RAS_PI:=--platform linux/arm64)
	echo $(BUILD_PLATFORM_RAS_PI)

.PHONY: docker-build-proxy
docker-build-proxy: eval-git-hash
	 DOCKER_BUILDKIT=1 docker build --build-arg GIT_HASH=$(GIT_HASH) $(BUILD_PLATFORM_RAS_PI) -t proxy-service:0.2.2 -t proxy-service:latest  .

.PHONY: docker-tag-k3d-registry-proxy
docker-tag-k3d-registry-proxy: docker-build-proxy
	docker tag proxy-service:latest localhost:54094/proxy-service:latest
	docker tag proxy-service:0.2.2 localhost:54094/proxy-service:0.2.2
	docker push localhost:54094/proxy-service:latest
	docker push localhost:54094/proxy-service:0.2.2

.PHONY: docker-tag-microk8s-registry-proxy
docker-tag-microk8s-registry-proxy: build-raspberry-pi docker-build-proxy
	docker tag proxy-service:latest registry.local:32000/proxy-service:latest
	docker tag proxy-service:0.2.2 registry.local:32000/proxy-service:0.2.2
	docker push registry.local:32000/proxy-service:latest
	docker push registry.local:32000/proxy-service:0.2.2


.PHONY: k3d-proxy-deploy-deployment
k3d-proxy-deploy-deployment:
	kubectl apply -f ../install/kubectl/proxy-service-k3d-deployment.yaml

.PHONY: k3d-proxy-undeploy-deployment
k3d-proxy-undeploy-deployment:
	kubectl delete -f ../install/kubectl/proxy-service-k3d-deployment.yaml

.PHONY: k3d-proxy-deploy-pod
k3d-proxy-deploy-pod:
	kubectl apply -f ../install/kubectl/proxy-service-k3d-pod.yaml

.PHONY: k3d-proxy-undeploy-pod
k3d-proxy-undeploy-pod:
	kubectl delete -f ../install/kubectl/proxy-service-k3d-pod.yaml

setup-proxy-properties:
	$(eval proxy_setup := GRPC_GO_LOG_SEVERITY_LEVEL=info GRPC_GO_LOG_VERBOSITY_LEVEL=99 NAMESPACE=no-namespace INSTANCE_NAME=proxy-service-1)

setup-proxy-docker-properties:
	$(eval proxy_setup := -e GRPC_GO_LOG_SEVERITY_LEVEL=info -e GRPC_GO_LOG_VERBOSITY_LEVEL=99 -e NAMESPACE=no-namespace -e INSTANCE_NAME=proxy-service-1)

.PHONY: docker-k3d-start
docker-k3d-start: setup-proxy-docker-properties
	docker run -d -p 9070:9070 $(proxy_setup) -e OTEL_LOCATION=otel-collector.local:8070 -e ALBUM_STORE_URL=http://localhost:9080  --name proxy-service proxy-service:0.1

.PHONY: docker-local-start
docker-local-start: setup-proxy-docker-properties
	 docker run -d -p 9070:9070 $(proxy_setup) -e OTEL_LOCATION=localhost:4327 -e ALBUM_STORE_URL=http://localhost:9080 --name proxy-service proxy-service:latest

.PHONY: local-start-k3d
local-start-k3d: build setup-proxy-properties
	$(proxy_setup) OTEL_LOCATION=otel-collector.local:8070 ALBUM_STORE_URL=http://album-service.album-service.svc.cluster.local:9080 ./proxy-service-bin

.PHONY: local-start-proxy
local-start-proxy: build setup-proxy-properties
	$(proxy_setup) OTEL_LOCATION=localhost:4327 ALBUM_STORE_URL=http://localhost:9080 ./proxy-service-bin

.PHONY: docker-stop
docker-stop:
	docker stop proxy-service;

.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out;
	go tool cover -html=coverage.out -o coverage.html;

.PHONY: generate-swagger
generate-swagger:
	go get -u github.com/swaggo/swag/cmd/swag
	go get -u github.com/swaggo/gin-swagger
	go get -u github.com/swaggo/files
	swag init -o api