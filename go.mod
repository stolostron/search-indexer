// Copyright Contributors to the Open Cluster Management project
module github.com/stolostron/search-indexer

go 1.23.0

require (
	github.com/doug-martin/goqu/v9 v9.19.0
	github.com/driftprogramming/pgxpoolmock v1.1.0
	github.com/golang/mock v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/jackc/pgconn v1.14.3
	github.com/jackc/pgproto3/v2 v2.3.3
	github.com/jackc/pgx/v4 v4.18.3
	github.com/pashagolub/pgxmock v1.8.0
	github.com/prometheus/client_golang v1.22.0
	github.com/stolostron/multicloud-operators-foundation v1.0.0
	github.com/stretchr/testify v1.10.0
	k8s.io/apimachinery v0.33.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.130.1
	open-cluster-management.io/api v0.16.2
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch v5.9.11+incompatible // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile 5a60cdf6a761 // indirect
	github.com/jackc/pgtype v1.14.4 // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/term v0.32.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.33.2 // indirect
	k8s.io/kube-openapi 9bd5c66d9911 // indirect
	k8s.io/utils 4c0f3b243397 // indirect
	sigs.k8s.io/controller-runtime v0.21.0 // indirect
	sigs.k8s.io/json cfa47c3a1cc8 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
	sigs.k8s.io/yaml v1.5.0 // indirect
)

replace (
	github.com/IBM-Cloud/terraform-provider-ibm => github.com/openshift/terraform-provider-ibm v1.76.2
	github.com/imdario/mergo => dario.cat/mergo v1.0.2
	github.com/metal3-io/baremetal-operator => github.com/openshift/cluster-baremetal-operator c6838e2773d0
	github.com/metal3-io/baremetal-operator/apis => github.com/metal3-io/baremetal-operator/apis v0.0.0-20220323083018-9bfb47657ba6
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils => github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.10.2
	github.com/openshift/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure 48ebcc043072
	github.com/openshift/hive/apis => github.com/openshift/hive/apis 016b52396058
	github.com/stolostron/multicloud-operators-foundation => github.com/stolostron/multicloud-operators-foundation v0.0.0-20220317080545-2ea99b88c0fd // indirect
	github.com/terraform-providers/terraform-provider-aws => github.com/hashicorp/terraform-provider-aws 229b588b9676
	github.com/terraform-providers/terraform-provider-ignition/v2 => github.com/community-terraform-providers/terraform-provider-ignition/v2 v2.5.1
	k8s.io/api => k8s.io/api v0.33.2
	k8s.io/client-go => k8s.io/client-go v0.33.2
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.10.3
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws de68d01bb2c1
	sigs.k8s.io/cluster-api-provider-azure => sigs.k8s.io/cluster-api-provider-azure v1.20.1
	sigs.k8s.io/cluster-api-provider-openstack => sigs.k8s.io/cluster-api-provider-openstack v0.12.4
)
