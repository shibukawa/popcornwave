// load.js drives benchserver from outside the process, which is the only way
// to get a requests-per-second number that is the server's rather than the
// server's and the client's together.
//
// Response bodies are discarded. The bodies are identical on both transports so
// keeping them would measure the same parsing twice, and discarding them leaves
// more of a shared machine to the thing under test.
//
//   k6 run -e BASE=http://127.0.0.1:8081 -e ROUTE=/api -e LABEL=nethttp \
//          -e OUT=/tmp/nethttp-api.json internal/transportbench/load.js
import http from 'k6/http'
import { check } from 'k6'

export const options = {
  discardResponseBodies: true,
  scenarios: {
    load: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 50),
      duration: __ENV.DURATION || '15s',
      gracefulStop: '2s',
    },
  },
}

export default function () {
  const response = http.get(`${__ENV.BASE}${__ENV.ROUTE}`)
  check(response, { 'answered 200': (r) => r.status === 200 })
}

// handleSummary writes one row rather than k6's report, because the runs are
// compared against each other and a table of six is easier to read than six
// reports.
export function handleSummary(data) {
  const metrics = data.metrics
  const duration = metrics.http_req_duration.values
  const row = {
    backend: __ENV.LABEL,
    route: __ENV.ROUTE,
    vus: Number(__ENV.VUS || 50),
    requests: metrics.http_reqs.values.count,
    rps: metrics.http_reqs.values.rate,
    failed_ratio: metrics.http_req_failed ? metrics.http_req_failed.values.rate : 0,
    med_ms: duration.med,
    p95_ms: duration['p(95)'],
    p99_ms: duration['p(99)'],
    max_ms: duration.max,
  }
  const out = {}
  out[__ENV.OUT] = JSON.stringify(row, null, 2) + '\n'
  return out
}
