// Copyright Contributors to the Open Cluster Management project
module github.com/stolostron/search-indexer

go 1.20

require (
	github.com/doug-martin/goqu/v9 v9.18.0
	github.com/driftprogramming/pgxpoolmock v1.1.0
	github.com/golang/mock v1.6.0
	github.com/gorilla/mux v1.8.0
	github.com/jackc/pgconn v1.14.0
	github.com/jackc/pgproto3/v2 v2.3.2
	github.com/jackc/pgx/v4 v4.18.1
	github.com/pashagolub/pgxmock v1.8.0
	github.com/prometheus/client_golang v1.15.1
	github.com/stolostron/multicloud-operators-foundation v1.0.0
	github.com/stretchr/testify v1.8.1
	k8s.io/apimachinery v0.27.2
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.100.1
	open-cluster-management.io/api v0.11.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.0 // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lib/pq v1.10.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.10.0 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.9.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.27.2 // indirect
	k8s.io/kube-openapi v0.0.0-20230515203736-54b630e78af5 // indirect
	k8s.io/utils v0.0.0-20230505201702-9f6742963106 // indirect
	sigs.k8s.io/controller-runtime v0.15.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	github.com/IBM-Cloud/terraform-provider-ibm => github.com/openshift/terraform-provider-ibm v1.26.2-openshift-2
	github.com/metal3-io/baremetal-operator => github.com/openshift/cluster-baremetal-operator v0.0.0-20220325053116-efa06caa3e66
	github.com/metal3-io/baremetal-operator/apis => github.com/metal3-io/baremetal-operator/apis v0.0.0-20220323083018-9bfb47657ba6
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils => github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.0.0-20220323083018-9bfb47657ba6
	github.com/openshift/cluster-api-provider-azure => github.com/openshift/cluster-api-provider-azure v1.2.1-0.20220325212758-d1c52034b92e
	github.com/openshift/hive/apis => github.com/openshift/hive/apis v0.0.0-20220325201640-a149ff297128
	github.com/stolostron/multicloud-operators-foundation => github.com/stolostron/multicloud-operators-foundation v0.0.0-20220317080545-2ea99b88c0fd // indirect
	github.com/terraform-providers/terraform-provider-aws => github.com/hashicorp/terraform-provider-aws v1.60.1-0.20220325205205-433af9276ab5
	github.com/terraform-providers/terraform-provider-ignition/v2 => github.com/community-terraform-providers/terraform-provider-ignition/v2 v2.1.3-0.20220105023746-b0f9e422dda5
	k8s.io/api => k8s.io/api v0.24.3
	k8s.io/client-go => k8s.io/client-go v0.24.3
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.2.0
	sigs.k8s.io/cluster-api-provider-aws => github.com/openshift/cluster-api-provider-aws v0.2.1-0.20220303094158-3c2daa272191
	sigs.k8s.io/cluster-api-provider-azure => sigs.k8s.io/cluster-api-provider-azure v1.2.1
	sigs.k8s.io/cluster-api-provider-openstack => sigs.k8s.io/cluster-api-provider-openstack v0.5.1-0.20220325161359-d73965086790
)
