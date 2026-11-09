import http from 'k6/http'
import { check } from 'k6'
import { Trend } from 'k6/metrics'
import encoding from 'k6/encoding'

const base = __ENV.BASE_URL || 'http://localhost:8091'
const clientID = __ENV.CLIENT_ID || 'sample-backend'
const secret = __ENV.CLIENT_SECRET || ''
const rps = Number(__ENV.RPS || 50)

const tokenLatency = new Trend('token_latency', true)

export const options = {
  scenarios: {
    token: {
      executor: 'constant-arrival-rate',
      rate: rps,
      timeUnit: '1s',
      duration: __ENV.DURATION || '30s',
      preAllocatedVUs: Math.max(10, Math.ceil(rps / 2)),
      maxVUs: Math.max(20, rps * 2),
    },
  },
  thresholds: {
    token_latency: ['p(95)<150'],
    http_req_failed: ['rate<0.01'],
  },
}

export default function () {
  const authorization = encoding.b64encode(`${clientID}:${secret}`)
  const response = http.post(
    `${base}/realms/default/token`,
    'grant_type=client_credentials&scope=openid',
    {
      headers: {
        'Content-Type': 'application/x-www-form-urlencoded',
        Authorization: `Basic ${authorization}`,
      },
      tags: { endpoint: 'token' },
    },
  )

  tokenLatency.add(response.timings.duration)
  check(response, { 'token ok': (result) => result.status === 200 })
}
