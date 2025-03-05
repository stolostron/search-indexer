# Copyright Contributors to the Open Cluster Management project


default::
	make help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'


setup: ## Generate ssl certificate for development.
	cd sslcert; openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -config req.conf -extensions 'v3_req'

setup-dev: ## Configure local environment to use the postgres instance on the dev cluster.
	@echo "Using current target cluster.\\n"
	@echo "$(shell oc cluster-info)"
	@echo "\\n1. [MANUAL STEP] Set these environment variables.\\n"
	export DB_NAME=$(shell oc get secret search-postgres -n open-cluster-management -o jsonpath='{.data.database-name}'|base64 -D)
	export DB_USER=$(shell oc get secret search-postgres -n open-cluster-management -o jsonpath='{.data.database-user}'|base64 -D)
	export DB_PASS=$(shell oc get secret search-postgres -n open-cluster-management -o jsonpath='{.data.database-password}'|base64 -D)
	@echo "\\n2. [MANUAL STEP] Start port forwarding.\\n"
	@echo "oc port-forward service/search-postgres -n open-cluster-management 5432:5432 \\n"

run: ## Run the service locally.
	go run -tags development main.go -v=3

.PHONY: lint
lint: ## Run lint and gosec tool.
	GOPATH=$(go env GOPATH)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "${GOPATH}/bin" v1.61.0
	CGO_ENABLED=1 GOGC=25 golangci-lint run --timeout=3m
	go mod tidy
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

CLUSTERS ?=10
RATE ?=1
HOST ?= $(shell oc get route search-indexer -o custom-columns=host:.spec.host --no-headers -n open-cluster-management --ignore-not-found=true --request-timeout='1s')
ifeq ($(strip $(HOST)),)
	CONFIGURATION_MSG = @echo \\n\\tThe search-indexer route was not found in the target cluster.\\n\
	\\tThis test will run against the local instance https://localhost:3010\\n\
	\\tIf you want to run this test against a cluster, create the route with make test-scale-setup\\n;
	
	HOST = localhost:3010
endif

show-metrics:
	curl -k https://localhost:3010/metrics

test-scale: check-locust ## Simulate multiple clusters posting data to the indexer. Defaults: CLUSTERS=10 RATE=1
	${CONFIGURATION_MSG}
	cd test; locust --headless --users ${CLUSTERS} --spawn-rate ${RATE} -H https://${HOST} -f locust-clusters.py --only-summary

test-scale-ui: check-locust ## Start Locust and open the web browser to drive scale tests.  Defaults: CLUSTERS=10 RATE=1
	${CONFIGURATION_MSG}
	open http://0.0.0.0:8085/
	cd test; locust --users ${CLUSTERS} --spawn-rate ${RATE} -H https://${HOST} -P 8085 -f locust-clusters.py --class-picker 

test-scale-setup: ## Creates the search-indexer route in the current target cluster.
	oc create route passthrough search-indexer --service=search-indexer -n open-cluster-management

check-locust: ## Checks if Locust is installed in the system.
ifeq (,$(shell which locust))
	@echo The scale tests require Locust.io, but locust was not found.
	@echo Install locust to continue. For more info visit: https://docs.locust.io/en/stable/installation.html
	exit 1
endif

