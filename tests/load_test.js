import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

// Custom metrics
const rateLimitedRequests = new Rate("rate_limited_requests");
const requestLatency = new Trend("request_latency_ms");

export const options = {
  scenarios: {
    // Scenario 1: Normal load
    normal_load: {
      executor: "constant-vus",
      vus: 10,
      duration: "30s",
      tags: { scenario: "normal" },
    },
    // Scenario 2: Spike — triggers rate limiting
    spike: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "10s", target: 50 },
        { duration: "20s", target: 200 },
        { duration: "10s", target: 0 },
      ],
      startTime: "35s", // starts after normal load
      tags: { scenario: "spike" },
    },
  },
  thresholds: {
    // httpbin.org slow — 2s threshold realistic for external upstream
    http_req_duration: ["p(95)<2000"],
    http_req_failed: ["rate<0.01"],
  },
};

export default function () {
  const start = Date.now();

  const res = http.get("http://localhost:8081/get", {
    headers: {
      "X-API-Key": `user-${__VU}`, // each VU = different user
    },
  });

  const latency = Date.now() - start;
  requestLatency.add(latency);

  // 429 = rate limited (expected behavior, not a failure)
  const isRateLimited = res.status === 429;
  rateLimitedRequests.add(isRateLimited);

  check(res, {
    "status is 200 or 429": (r) => r.status === 200 || r.status === 429,
    "response time < 500ms": (r) => r.timings.duration < 500,
    "has rate limit header": (r) =>
      r.headers["X-Ratelimit-Limit"] !== undefined,
  });

  sleep(0.1);
}

export function handleSummary(data) {
  return {
    "tests/results/summary.json": JSON.stringify(data, null, 2),
  };
}
