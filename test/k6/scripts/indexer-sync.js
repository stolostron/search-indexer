import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const INDEXER_HOST = __ENV.INDEXER_HOST || 'localhost:3010';
const BASE_URL = `https://${INDEXER_HOST}`;

const fullStateTemplate = JSON.parse(open('../payloads/cluster-5k.json'));
const updateTemplate = JSON.parse(open('../payloads/update-pod.json'));

export const options = {
  vus: __ENV.TEST_VUS ? parseInt(__ENV.TEST_VUS) : 2,
  duration: __ENV.TEST_DURATION || '60s',
  insecureSkipTLSVerify: true,
  thresholds: {
    'http_req_duration{sync_type:full}': ['p(95)<30000'],
    'http_req_duration{sync_type:delta}': ['p(95)<5000'],
    http_req_failed: ['rate<0.1'],
  },
};

function clusterName() {
  return `k6-cluster-${__VU}`;
}

function buildFullStatePayload() {
  const name = clusterName();
  const raw = JSON.stringify(fullStateTemplate);
  return raw.replaceAll('<<CLUSTER_NAME>>', name);
}

function buildUpdatePayload() {
  const name = clusterName();
  const uid = uuidv4();
  const raw = JSON.stringify(updateTemplate);
  return raw.replaceAll('<<CLUSTER_NAME>>', name).replaceAll('<<UID>>', uid);
}

function sendSync(payload, isFullState) {
  const name = clusterName();
  const url = `${BASE_URL}/aggregator/clusters/${name}/sync`;
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-Overwrite-State': isFullState ? 'true' : 'false',
    },
    tags: {
      sync_type: isFullState ? 'full' : 'delta',
      name: isFullState ? 'sync:full' : 'sync:delta',
    },
  };

  const res = http.post(url, payload, params);
  check(res, {
    'status is 200': (r) => r.status === 200,
  });
  return res;
}

export function setup() {
  // Initial full-state sync for all VUs before the test loop starts.
  // k6 setup runs once, so we sync VU 1's cluster as a smoke test.
  const name = 'k6-cluster-1';
  const raw = JSON.stringify(fullStateTemplate).replaceAll('<<CLUSTER_NAME>>', name);
  const url = `${BASE_URL}/aggregator/clusters/${name}/sync`;
  const res = http.post(url, raw, {
    headers: {
      'Content-Type': 'application/json',
      'X-Overwrite-State': 'true',
    },
    tags: { sync_type: 'full', name: 'sync:setup' },
  });
  check(res, { 'setup sync succeeded': (r) => r.status === 200 });
}

export default function () {
  // On first iteration for this VU, send a full state sync.
  if (__ITER === 0) {
    const payload = buildFullStatePayload();
    sendSync(payload, true);
    sleep(5);
    return;
  }

  // Weighted choice: 10:1 ratio of delta updates to full resyncs.
  const roll = Math.random();
  if (roll < 10 / 11) {
    const payload = buildUpdatePayload();
    sendSync(payload, false);
  } else {
    const payload = buildFullStatePayload();
    sendSync(payload, true);
  }

  // Random wait between 5-30s (shortened from locust's 5-300s for PoC speed).
  sleep(Math.random() * 25 + 5);
}
