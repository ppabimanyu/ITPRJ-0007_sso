import http from 'k6/http'
import { check, sleep } from 'k6'
import { Trend } from 'k6/metrics'

const base = __ENV.BASE_URL || 'http://localhost:8091'
const email = __ENV.LOGIN_EMAIL || 'user@example.com'
const password = __ENV.LOGIN_PASSWORD || 'E2E-User-Passw0rd!'
const origin = __ENV.ORIGIN || base
const vus = Number(__ENV.VUS || 10)

const loginLatency = new Trend('login_latency', true)

export const options = {
  scenarios: {
    login: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: vus },
        { duration: '20s', target: vus },
        { duration: '5s', target: 0 },
      ],
    },
  },
  thresholds: {
    login_latency: ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
}

export default function () {
  const csrf = http.get(`${base}/api/v1/auth/csrf`, { tags: { endpoint: 'csrf' } })
  const token = csrf.json('csrf_token')

  const response = http.post(
    `${base}/api/v1/auth/login`,
    JSON.stringify({ email, password, remember_me: false }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': token,
        Origin: origin,
      },
      tags: { endpoint: 'login' },
    },
  )

  loginLatency.add(response.timings.duration)
  check(response, { 'login ok': (result) => result.status === 200 })
  sleep(1)
}
