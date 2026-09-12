import ws from 'k6/ws';
import { check } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const API_HOST = __ENV.API_HOST || 'localhost:4010';
const API_TOKEN = __ENV.API_TOKEN || '';
const WS_URL = `wss://${API_HOST}/searchapi/graphql`;
const SUB_DURATION = __ENV.SUB_DURATION ? parseInt(__ENV.SUB_DURATION) : 30;

export const options = {
  vus: __ENV.TEST_VUS ? parseInt(__ENV.TEST_VUS) : 2,
  duration: __ENV.TEST_DURATION || '60s',
  insecureSkipTLSVerify: true,
};

const wsMessages = new Counter('ws_messages_received');
const wsLatency = new Trend('ws_inter_message_latency', true);
const wsConnectTime = new Trend('ws_connect_time', true);

const KINDS = ['Pod', 'Deployment', 'Service', 'ConfigMap', 'Secret'];

function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

export default function () {
  const subId = `sub-${__VU}-${__ITER}`;
  const filterKind = pick(KINDS);
  let lastMessageTime = null;
  let ackReceived = false;
  const connectStart = Date.now();

  const res = ws.connect(WS_URL, { headers: { 'Sec-WebSocket-Protocol': 'graphql-transport-ws' } }, function (socket) {
    socket.on('open', function () {
      const elapsed = Date.now() - connectStart;
      wsConnectTime.add(elapsed);

      socket.send(JSON.stringify({
        type: 'connection_init',
        payload: { Authorization: `Bearer ${API_TOKEN}` },
      }));
    });

    socket.on('message', function (msg) {
      const data = JSON.parse(msg);

      if (data.type === 'connection_ack') {
        ackReceived = true;
        socket.send(JSON.stringify({
          id: subId,
          type: 'subscribe',
          payload: {
            query: `subscription ($input: SearchInput) {
              watch(input: $input) { uid operation newData timestamp }
            }`,
            variables: {
              input: {
                filters: [{ property: 'kind', values: [filterKind] }],
              },
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
        return;
      }

      if (data.type === 'complete') {
        socket.close();
        return;
      }

      // ka (keep-alive) and pong messages are expected, ignore them.
    });

    socket.on('error', function (e) {
      console.error(`WebSocket error: ${e.error()}`);
    });

    socket.setTimeout(function () {
      socket.send(JSON.stringify({ id: subId, type: 'complete' }));
      socket.close();
    }, SUB_DURATION * 1000);
  });

  check(res, {
    'ws status is 101': (r) => r && r.status === 101,
  });
}
