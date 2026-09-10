import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// --- Config ---
const INDEXER_HOST = __ENV.INDEXER_HOST || 'localhost:3010';
const API_HOST = __ENV.API_HOST || 'localhost:4010';
const API_TOKEN = __ENV.API_TOKEN || '';
const SUB_DURATION = __ENV.SUB_DURATION ? parseInt(__ENV.SUB_DURATION) : 30;

// --- Payloads (loaded at init time) ---
const fullStateTemplate = JSON.parse(open('../payloads/cluster-5k.json'));
const updateTemplate = JSON.parse(open('../payloads/update-pod.json'));

// --- Custom metrics ---
const wsMessages = new Counter('ws_messages_received');
const wsLatency = new Trend('ws_inter_message_latency', true);
const wsConnectTime = new Trend('ws_connect_time', true);

// --- Shared helpers ---
const KEYWORDS = ['apiserver', 'nginx', 'redis', 'etcd', 'openshift', 'kube-system'];
const NAMESPACES = ['default', 'kube-system', 'openshift-monitoring', 'open-cluster-management'];
const KINDS = ['Pod', 'Deployment', 'Service', 'ConfigMap', 'Secret', 'ReplicaSet'];

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

function apiHeaders() {
  return {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${API_TOKEN}`,
  };
}

const GQL_SEARCH = `query q($input: [SearchInput]) {
  searchResult: search(input: $input) { items count __typename }
}`;
const GQL_COMPLETE = `query q($property: String!, $query: SearchInput, $limit: Int) {
  searchComplete(property: $property, query: $query, limit: $limit)
}`;
const GQL_RELATED = `query q($input: [SearchInput]) {
  searchResult: search(input: $input) { related { kind count __typename } __typename }
}`;

// --- Options ---
export const options = {
  insecureSkipTLSVerify: true,
  scenarios: {
    indexer_load: {
      executor: 'constant-vus',
      exec: 'indexerSync',
      vus: __ENV.INDEXER_VUS ? parseInt(__ENV.INDEXER_VUS) : 3,
      duration: __ENV.TEST_DURATION || '120s',
      tags: { scenario: 'indexer' },
    },
    api_queries: {
      executor: 'constant-vus',
      exec: 'apiQuery',
      vus: __ENV.API_VUS ? parseInt(__ENV.API_VUS) : 5,
      duration: __ENV.TEST_DURATION || '120s',
      startTime: '10s',
      tags: { scenario: 'api' },
    },
    subscriptions: {
      executor: 'constant-vus',
      exec: 'subscription',
      vus: __ENV.SUB_VUS ? parseInt(__ENV.SUB_VUS) : 2,
      duration: __ENV.TEST_DURATION || '120s',
      startTime: '10s',
      tags: { scenario: 'subscriptions' },
    },
  },
  thresholds: {
    'http_req_duration{scenario:indexer}': ['p(95)<30000'],
    'http_req_duration{scenario:api}': ['p(95)<5000'],
    http_req_failed: ['rate<0.1'],
  },
};

// ============================================================
// Scenario: Indexer Sync
// ============================================================
export function indexerSync() {
  const name = `k6-cluster-${__VU}`;
  const baseUrl = `https://${INDEXER_HOST}/aggregator/clusters/${name}/sync`;

  if (__ITER === 0) {
    const payload = JSON.stringify(fullStateTemplate).replaceAll('<<CLUSTER_NAME>>', name);
    const res = http.post(baseUrl, payload, {
      headers: { 'Content-Type': 'application/json', 'X-Overwrite-State': 'true' },
      tags: { sync_type: 'full', name: 'sync:full' },
    });
    check(res, { 'sync:full status 200': (r) => r.status === 200 });
    sleep(5);
    return;
  }

  const roll = Math.random();
  if (roll < 10 / 11) {
    const uid = uuidv4();
    const payload = JSON.stringify(updateTemplate)
      .replaceAll('<<CLUSTER_NAME>>', name)
      .replaceAll('<<UID>>', uid);
    const res = http.post(baseUrl, payload, {
      headers: { 'Content-Type': 'application/json', 'X-Overwrite-State': 'false' },
      tags: { sync_type: 'delta', name: 'sync:delta' },
    });
    check(res, { 'sync:delta status 200': (r) => r.status === 200 });
  } else {
    const payload = JSON.stringify(fullStateTemplate).replaceAll('<<CLUSTER_NAME>>', name);
    const res = http.post(baseUrl, payload, {
      headers: { 'Content-Type': 'application/json', 'X-Overwrite-State': 'true' },
      tags: { sync_type: 'full', name: 'sync:full' },
    });
    check(res, { 'sync:full status 200': (r) => r.status === 200 });
  }

  sleep(Math.random() * 25 + 5);
}

// ============================================================
// Scenario: API Queries
// ============================================================
function graphql(body, queryType) {
  const res = http.post(
    `https://${API_HOST}/searchapi/graphql`,
    JSON.stringify(body),
    {
      headers: apiHeaders(),
      tags: { query_type: queryType, name: `query:${queryType}` },
    }
  );
  check(res, {
    [`query:${queryType} status 200`]: (r) => r.status === 200,
  });
  return res;
}

const QUERY_TASKS = [
  ...Array(10).fill(() => graphql({
    query: GQL_SEARCH,
    variables: { input: [{ keywords: [pick(KEYWORDS)], limit: 1000 }] },
  }, 'keyword')),
  ...Array(5).fill(() => graphql({
    query: GQL_SEARCH,
    variables: { input: [{ filters: [{ property: 'kind', values: [pick(KINDS)] }, { property: 'namespace', values: [pick(NAMESPACES)] }], limit: 1000 }] },
  }, 'filter')),
  ...Array(3).fill(() => graphql({
    query: GQL_SEARCH,
    variables: { input: [{ filters: [{ property: 'kind', values: ['Pod'] }] }, { filters: [{ property: 'kind', values: ['Deployment'] }] }] },
  }, 'count')),
  ...Array(3).fill(() => graphql({
    query: GQL_COMPLETE,
    variables: { property: 'name', limit: 1000 },
  }, 'autocomplete')),
  ...Array(2).fill(() => graphql({
    query: GQL_RELATED,
    variables: { input: [{ filters: [{ property: 'name', values: ['apiserver'] }], limit: 1000 }] },
  }, 'related')),
];

export function apiQuery() {
  const task = QUERY_TASKS[Math.floor(Math.random() * QUERY_TASKS.length)];
  task();
  sleep(Math.random() * 4 + 1);
}

// ============================================================
// Scenario: WebSocket Subscriptions
// ============================================================
export function subscription() {
  const subId = `sub-${__VU}-${__ITER}`;
  const filterKind = pick(KINDS);
  let lastMessageTime = null;
  const connectStart = Date.now();

  const res = ws.connect(
    `wss://${API_HOST}/searchapi/graphql`,
    { headers: { 'Sec-WebSocket-Protocol': 'graphql-transport-ws' } },
    function (socket) {
      socket.on('open', function () {
        wsConnectTime.add(Date.now() - connectStart);
        socket.send(JSON.stringify({
          type: 'connection_init',
          payload: { Authorization: `Bearer ${API_TOKEN}` },
        }));
      });

      socket.on('message', function (msg) {
        const data = JSON.parse(msg);

        if (data.type === 'connection_ack') {
          socket.send(JSON.stringify({
            id: subId,
            type: 'subscribe',
            payload: {
              query: `subscription ($input: SearchInput) {
                watch(input: $input) { uid operation newData timestamp }
              }`,
              variables: {
                input: { filters: [{ property: 'kind', values: [filterKind] }] },
              },
            },
          }));
          lastMessageTime = Date.now();
          return;
        }

        if (data.type === 'next') {
          wsMessages.add(1);
          const now = Date.now();
          if (lastMessageTime) {
            wsLatency.add(now - lastMessageTime);
          }
          lastMessageTime = now;
          return;
        }

        if (data.type === 'error') {
          console.error(`Subscription error: ${JSON.stringify(data.payload)}`);
          socket.close();
        }
      });

      socket.on('error', function (e) {
        console.error(`WebSocket error: ${e.error()}`);
      });

      socket.setTimeout(function () {
        socket.send(JSON.stringify({ id: subId, type: 'complete' }));
        socket.close();
      }, SUB_DURATION * 1000);
    }
  );

  check(res, { 'ws status 101': (r) => r && r.status === 101 });
}
