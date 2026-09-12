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
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "${GOPATH}/bin" v2.9.0
	CGO_ENABLED=1 GOGC=25 golangci-lint run --timeout=3m
	go mod tidy
	gosec ./...

.PHONY: test
test: ## Run unit tests.
	go test ./... -failfast

coverage: ## Run unit tests and show code coverage.
	go test ./... -failfast -v -coverprofile cover.out
	go tool cover -html=cover.out -o=cover.html
	open cover.html

docker-build: ## Build the docker image.
	docker build -f Dockerfile . -t search-indexer

podman-build: ## Build the docker image.
	podman build -f Dockerfile . -t search-indexer

test-send: ## Sends a simulated request for testing using cURL.
	curl -k -H "X-Overwrite-State: true" -d "@pkg/server/mocks/clusterA.json" -X POST https://localhost:3010/aggregator/clusters/clusterA/sync

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

# --- k6 scale testing ---
# Note: env vars prefixed K6_ are reserved by k6 as CLI overrides.
# Use TEST_ prefix for our variables to avoid collisions.
TEST_VUS ?= 2
TEST_DURATION ?= 60s
INDEXER_HOST ?= $(shell oc get route search-indexer -n open-cluster-management -o jsonpath='{.spec.host}' 2>/dev/null || echo "localhost:3010")
API_HOST ?= $(shell oc get route search-api -n open-cluster-management -o jsonpath='{.spec.host}' 2>/dev/null || echo "localhost:4010")
API_TOKEN ?= $(shell oc whoami -t 2>/dev/null)
THANOS_HOST ?= $(shell oc get route thanos-querier -n openshift-monitoring -o jsonpath='{.spec.host}' 2>/dev/null || echo "localhost:9091")
K6_INFLUX ?= http://localhost:8086/k6
COMPOSE ?= $(shell which podman-compose 2>/dev/null || which docker-compose 2>/dev/null || echo "podman compose")

check-k6: ## Checks if k6 is installed in the system.
ifeq (,$(shell which k6))
	@echo k6 is required but not found.
	@echo Install k6 to continue. For more info visit: https://grafana.com/docs/k6/latest/set-up/install-k6/
	exit 1
endif

test-k6-setup: ## Start Grafana + InfluxDB monitoring stack for k6.
	THANOS_HOST=$(THANOS_HOST) API_TOKEN=$(API_TOKEN) envsubst < test/k6/grafana-datasource.yml.tpl > test/k6/grafana-datasource.yml
	cd test/k6 && $(COMPOSE) up -d
	@echo "Grafana available at http://localhost:3000 (admin/admin)"

test-k6-indexer: check-k6 ## Run k6 indexer sync load test.
	TEST_VUS=$(TEST_VUS) TEST_DURATION=$(TEST_DURATION) INDEXER_HOST=$(INDEXER_HOST) \
		k6 run --out influxdb=$(K6_INFLUX) test/k6/scripts/indexer-sync.js

test-k6-api: check-k6 ## Run k6 API query load test.
	TEST_VUS=$(TEST_VUS) TEST_DURATION=$(TEST_DURATION) API_HOST=$(API_HOST) API_TOKEN=$(API_TOKEN) \
		k6 run --out influxdb=$(K6_INFLUX) test/k6/scripts/api-queries.js

test-k6-subscriptions: check-k6 ## Run k6 WebSocket subscription load test.
	TEST_VUS=$(TEST_VUS) TEST_DURATION=$(TEST_DURATION) API_HOST=$(API_HOST) API_TOKEN=$(API_TOKEN) \
		k6 run --out influxdb=$(K6_INFLUX) test/k6/scripts/subscriptions.js

test-k6-combined: check-k6 ## Run all k6 load tests simultaneously.
	INDEXER_HOST=$(INDEXER_HOST) API_HOST=$(API_HOST) API_TOKEN=$(API_TOKEN) \
	TEST_DURATION=$(TEST_DURATION) \
		k6 run --out influxdb=$(K6_INFLUX) test/k6/scripts/combined.js

test-k6-teardown: ## Stop and remove k6 monitoring stack.
	cd test/k6 && $(COMPOSE) down -v

