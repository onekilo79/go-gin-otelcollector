.PHONY: docker-compose-full-start
docker-compose-full-start: docker-build-album
	docker-compose -f ./install/docker/docker-compose.yaml up -d --remove-orphans;

.PHONY: build
build:
	go mod tidy;
	go get;
	go clean;
	go build -ldflags "-X main.version=0.1 -X main.gitHash=`git rev-parse --short HEAD`" -v -o album-service-bin

.PHONY: test
test:
	go test -v -json > test-results.json

.PHONY: test-benchmark
test-benchmark:
	go test -bench=. -count 2 -run=^# -benchmem

.PHONY: skaffold-helm-repos
skaffold-helm-repos:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts;
	helm repo add jaegertracing       	https://jaegertracing.github.io/helm-charts;
	helm repo add open-telemetry      	https://open-telemetry.github.io/opentelemetry-helm-charts;
	helm repo add nginx-stable        	https://helm.nginx.com/stable;
	helm repo add jaeger-all-in-one   	https://raw.githubusercontent.com/hansehe/jaeger-all-in-one/master/helm/charts;
	helm repo add grafana             	https://grafana.github.io/helm-charts;
	helm repo add istio               	https://istio-release.storage.googleapis.com/charts;
	helm repo add kiali 				https://kiali.org/helm-charts
	helm repo add argo                	https://argoproj.github.io/argo-helm;
	helm repo update;

.PHONY: skaffold-microk8s-dev
skaffold-microk8s-dev:
	skaffold dev -p microk8s -f install/skaffold-microk8s.yaml

.PHONY: skaffold-microk8s-infra-dev
skaffold-microk8s-infra-dev:
	skaffold dev -p microk8s -f install/skaffold-microk8s-infra.yaml

.PHONY: skaffold-k3d-dev
skaffold-k3d-dev:
	skaffold dev -p k3d -f install/skaffold-k3d.yaml

.PHONY: skaffold-k3d-infra-dev
skaffold-k3d-infra-dev:
	skaffold dev -p k3d -f install/skaffold-k3d-infra.yaml


set-local-test:
	$(eval url_value := http://localhost:9080)

set-k3d-album-test:
	$(eval url_value := http://album-service.local:8070)

set-k3d-proxy-test:
	$(eval url_value := http://proxy-service.local:8070)

set-local-proxy-test:
	$(eval url_value := http://localhost:9070)

.PHONY: local-test
local-test: set-local-test run-tests
	curl --location --request GET '$(url_value)/v3/api-docs';

.PHONY: local-proxy-test
local-proxy-test: set-local-proxy-test run-tests

.PHONY: k3d-proxy-test
k3d-proxy-test: set-k3d-proxy-test run-tests

.PHONY: k3d-album-test
k3d-album-test: set-k3d-album-test run-tests
	curl --location --request GET '$(url_value)/v3/api-docs';

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
	curl --write-out '%{http_code}' -s -S --output /dev/null --location --request GET '$(url_value)/metrics';

.PHONY: eval-git-hash
eval-git-hash:
	$(eval GIT_HASH:= $(shell git rev-parse --short HEAD))
	echo $(GIT_HASH)

.PHONY: build-raspberry-pi
build-raspberry-pi:
	$(eval BUILD_PLATFORM_RAS_PI:=--platform linux/arm64)
	echo $(BUILD_PLATFORM_RAS_PI)

.PHONY: docker-build-album
docker-build-album: eval-git-hash
	DOCKER_BUILDKIT=1 docker build --build-arg GIT_HASH=$(GIT_HASH) $(BUILD_PLATFORM_RAS_PI) -t album-service:0.2.2 -t album-service:latest .

.PHONY: docker-tag-k3d-registry-album
docker-tag-k3d-registry-album: docker-build-album
	docker tag album-service:latest localhost:54094/album-service:latest;
	docker tag album-service:0.2.2 localhost:54094/album-service:0.2.2;
	docker push localhost:54094/album-service:latest;
	docker push localhost:54094/album-service:0.2.2;

.PHONY: docker-tag-microk8s-registry-album
docker-tag-microk8s-registry-album: build-raspberry-pi docker-build-album
	docker tag album-service:latest registry.local:32000/album-service:latest;
	docker tag album-service:0.2.2 registry.local:32000/album-service:0.2.2;
	docker push registry.local:32000/album-service:latest;
	docker push registry.local:32000/album-service:0.2.2;

.PHONY: k3d-album-deploy-deployment
k3d-album-deploy-deployment: docker-tag-k3d-registry-album
	kubectl apply -f ./install/kubectl/album-service-k3d-deployment.yaml

.PHONY: k3d-album-undeploy-deployment
k3d-album-undeploy-deployment:
	kubectl delete -f ./install/kubectl/album-service-k3d-deployment.yaml

.PHONY: k3d-album-deploy-pod
k3d-album-deploy-pod: docker-tag-k3d-registry-album
	kubectl apply -f ./install/kubectl/album-service-k3d-pod.yaml

.PHONY: k3d-album-undeploy-pod
k3d-album-undeploy-pod:
	kubectl delete -f ./install/kubectl/album-service-k3d-pod.yaml

setup-album-properties:
	$(eval album_setup := NAMESPACE=no-namespace INSTANCE_NAME=album-service-1)

setup-album-docker-properties:
	$(eval album_setup := -e NAMESPACE=no-namespace -e INSTANCE_NAME=album-service-1)

.PHONY: docker-k3d-start
docker-k3d-start: docker-build-album setup-album-docker-properties
	docker run -d -p 9080:9080 $(album_setup) -e OTEL_LOCATION=otel-collector.local:8070 --name album-service album-service:0.1

.PHONY: docker-local-start
docker-local-start: docker-build-album setup-album-docker-properties
	docker run -d -p 9080:9080 $(album_setup) -e OTEL_LOCATION=localhost:4327 --name album-service-local album-service:0.1

.PHONY: local-start-k3d
local-start-k3d: build setup-album-properties
	$(album_setup) OTEL_LOCATION=otel-collector.local:8070 ./album-service-bin

.PHONY: local-start
local-start: build setup-album-properties
	$(album_setup) OTEL_LOCATION=localhost:4327 ./album-service-bin

.PHONY: docker-local-stop
docker-local-stop:
	docker stop album-service-local;

.PHONY: docker-compose-full-stop
docker-compose-full-stop:
	docker-compose -f ./install/docker/docker-compose.yaml down;

.PHONY: docker-compose-limited-start
docker-compose-limited-start:
	docker-compose -f ./install/docker/docker-compose-limited.yaml up -d --remove-orphans;

.PHONY: docker-compose-limited-stop
docker-compose-limited-stop:
	docker-compose -f ./install/docker/docker-compose-limited.yaml down;

.PHONY: k3d-cluster-create
k3d-cluster-create:
	k3d cluster create k3s-default --config ./install/k3d-config.yaml

.PHONY: k3d-cluster-delete
k3d-cluster-delete:
	k3d cluster delete k3s-default;

.PHONY: coverage
coverage:
	go test -coverprofile=coverage.out;
	go tool cover -html=coverage.out -o coverage.html;

.PHONY: generate-swagger
generate-swagger:
	go get -u github.com/swaggo/swag/cmd/swag
	go get -u github.com/swaggo/gin-swagger
	go get -u github.com/swaggo/files
	swag init -o api --exclude proxy