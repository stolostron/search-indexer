# Copyright Contributors to the Open Cluster Management project


default::
	make help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'


setup: ## Generate ssl certificate for development.
	cd sslcert; openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -config req.conf -extensions 'v3_req'

run: ## Run the service locally.
	go run -tags development main.go -v=9

.PHONY: lint
lint: ## Run lint and gosec tool.
	go mod tidy
	golangci-lint run
	gosec ./...

.PHONY: test
test: ## Run unit tests.
	go test ./... -v -coverprofile cover.out

coverage: test ## Run unit tests and show code coverage.
	go tool cover -html=cover.out -o=cover.html
	open cover.html

docker-build: ## Build the docker image.
	docker build -f Dockerfile . -t search-indexer


test-send: ## Sends a simulated request for testing using cURL.
	curl -k -d "@pkg/server/mocks/clusterA.json" -X POST https://localhost:3010/aggregator/clusters/clusterA/sync

N_CLUSTERS ?=2
test-scale: check-locust ## Sends multiple simulated requests for testing using Locust. Use N_CLUSTERS to change the number of simulated clusters.
	cd test; locust --headless --users ${N_CLUSTERS} --spawn-rate ${N_CLUSTERS} -H https://localhost:3010 -f locust-clusters.py

test-scale-ui: check-locust ## Start Locust and opens the UI to drive scale tests.
	open http://0.0.0.0:8089/
	cd test; locust --users ${N_CLUSTERS} --spawn-rate ${N_CLUSTERS} -H https://localhost:3010 -f locust-clusters.py

check-locust: ## Checks if Locust is installed in the system.
ifeq (,$(shell which locust))
	@echo The scale tests require Locust.io, but locust was not found.
	@echo Install locust to continue. For more info visit: https://docs.locust.io/en/stable/installation.html
	exit 1
endif

