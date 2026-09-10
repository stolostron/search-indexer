import http from 'k6/http';
import { check, sleep } from 'k6';

const API_HOST = __ENV.API_HOST || 'localhost:4010';
const API_TOKEN = __ENV.API_TOKEN || '';
const BASE_URL = `https://${API_HOST}/searchapi/graphql`;

export const options = {
  vus: __ENV.TEST_VUS ? parseInt(__ENV.TEST_VUS) : 5,
  duration: __ENV.TEST_DURATION || '60s',
  insecureSkipTLSVerify: true,
  thresholds: {
    'http_req_duration{query_type:keyword}': ['p(95)<5000'],
    'http_req_duration{query_type:filter}': ['p(95)<5000'],
    http_req_failed: ['rate<0.1'],
  },
};

const QUERIES = {
  search: `query q($input: [SearchInput]) {
    searchResult: search(input: $input) { items count __typename }
  }`,
  searchComplete: `query q($property: String!, $query: SearchInput, $limit: Int) {
    searchComplete(property: $property, query: $query, limit: $limit)
  }`,
  searchRelatedCount: `query q($input: [SearchInput]) {
    searchResult: search(input: $input) { related { kind count __typename } __typename }
  }`,
  searchRelatedItems: `query q($input: [SearchInput]) {
    searchResult: search(input: $input) { related { kind items __typename } __typename }
  }`,
};

const KEYWORDS = ['apiserver', 'nginx', 'redis', 'etcd', 'openshift', 'kube-system'];
const NAMESPACES = ['default', 'kube-system', 'openshift-monitoring', 'open-cluster-management'];
const KINDS = ['Pod', 'Deployment', 'Service', 'ConfigMap', 'Secret', 'ReplicaSet'];

function headers() {
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${API_TOKEN}`,
  };
}

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function graphql(body, queryType) {
  const params = {
    headers: headers(),
    tags: { query_type: queryType, name: `query:${queryType}` },
  };
  const res = http.post(BASE_URL, JSON.stringify(body), params);
  check(res, {
    'status is 200': (r) => r.status === 200,
    'no errors': (r) => {
      try { return !JSON.parse(r.body).errors; } catch { return false; }
    },
  });
  return res;
}

function keywordSearch() {
  graphql({
    query: QUERIES.search,
    variables: { input: [{ keywords: [pick(KEYWORDS)], limit: 1000 }] },
  }, 'keyword');
}

function filterSearch() {
  graphql({
    query: QUERIES.search,
    variables: {
      input: [{
        filters: [
          { property: 'kind', values: [pick(KINDS)] },
          { property: 'namespace', values: [pick(NAMESPACES)] },
        ],
        limit: 1000,
      }],
    },
  }, 'filter');
}

function countSearch() {
  graphql({
    query: QUERIES.search,
    variables: {
      input: [
        { filters: [{ property: 'kind', values: ['Pod'] }] },
        { filters: [{ property: 'kind', values: ['Deployment'] }] },
      ],
    },
  }, 'count');
}

function autocomplete() {
  graphql({
    query: QUERIES.searchComplete,
    variables: { property: 'name', limit: 1000 },
  }, 'autocomplete');
}

function relatedCount() {
  graphql({
    query: QUERIES.searchRelatedCount,
    variables: {
      input: [{ filters: [{ property: 'name', values: ['apiserver'] }], limit: 1000 }],
    },
  }, 'related_count');
}

function relatedItems() {
  graphql({
    query: QUERIES.searchRelatedItems,
    variables: {
      input: [{ filters: [{ property: 'kind', values: ['Pod'] }], limit: 100 }],
    },
  }, 'related_items');
}

const WEIGHTED_TASKS = [
  ...Array(10).fill(keywordSearch),
  ...Array(5).fill(filterSearch),
  ...Array(3).fill(countSearch),
  ...Array(3).fill(autocomplete),
  ...Array(2).fill(relatedCount),
  ...Array(1).fill(relatedItems),
];

export default function () {
  const task = WEIGHTED_TASKS[Math.floor(Math.random() * WEIGHTED_TASKS.length)];
  task();
  sleep(Math.random() * 4 + 1);
}
