// Read load against one todo service. Run it twice, once per BASE_URL, with
// the same seeded table behind both.
//
// The load is read-only on purpose. Writes would grow the table during the run,
// so the second service measured would be answering a larger query than the
// first and the comparison would drift. The row count is fixed by load.sh
// before either service starts.
//
//   k6 run -e BASE_URL=http://127.0.0.1:8081 load.js
import http from 'k6/http';
import { check } from 'k6';
import { Trend } from 'k6/metrics';

const base = __ENV.BASE_URL;

// Separate trends per route, because an HTML render and a JSON encode are the
// two different things this comparison is about and one blended latency would
// hide whichever moved.
const htmlLatency = new Trend('html_latency', true);
const jsonLatency = new Trend('json_latency', true);

export const options = {
  scenarios: {
    // Closed model: a fixed number of clients, each issuing the next request as
    // soon as the last returns. Throughput is then the server's answer rather
    // than a rate the generator imposed.
    read: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 20),
      duration: __ENV.DURATION || '30s',
      gracefulStop: '5s',
    },
  },
  // A run that breaches these is reported rather than quietly averaged in.
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
  summaryTrendStats: ['avg', 'med', 'p(95)', 'p(99)', 'max'],
};

// A browser asking for a page sends Accept: text/html, and the framework issues
// a CSRF token only for a request that looks like a document — an API read stays
// session-free. A load generator that omits the header is not exercising the
// page path, and the form render fails for want of a token.
const documentHeaders = { headers: { Accept: 'text/html,application/xhtml+xml' } };

export default function () {
  const html = http.get(`${base}/`, documentHeaders);
  htmlLatency.add(html.timings.duration);
  check(html, {
    'html 200': (r) => r.status === 200,
    'html has list': (r) => r.body.includes('class="count"'),
  });

  const json = http.get(`${base}/api/todos`);
  jsonLatency.add(json.timings.duration);
  check(json, {
    'json 200': (r) => r.status === 200,
    'json has todos': (r) => r.body.includes('"todos"'),
  });
}
