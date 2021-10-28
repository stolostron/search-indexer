# Copyright Contributors to the Open Cluster Management project


default::
	make help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'


setup: ## Generate ssl certificate for development.
	cd sslcert; openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -config req.conf -extensions 'v3_req'

run: ## Run the service locally.
	go run main.go -v=9

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
test-scale: ## Sends multiple simulated requests for testing using Locust. Use N_CLUSTERS to change the number of simulated clusters.
	cd test; locust --headless --users ${N_CLUSTERS} --spawn-rate ${N_CLUSTERS} -H https://localhost:3010 -f locust-clusters.py

test-scale-ui: ## Start Locust and opens the UI to drive scale tests.
	cd test; locust -f locust-clusters.py

