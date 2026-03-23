import http from 'k6/http';
import { check, sleep } from 'k6';
import { generateTransaction } from './data.js';

export const options = {
    stages: [
        { duration: '10s', target: 10 },   // normal
        { duration: '5s',  target: 500 },   // spike ramp
        { duration: '10s', target: 500 },   // spike sustain
        { duration: '5s',  target: 10 },    // spike drop
        { duration: '10s', target: 10 },    // recovery
    ],
    thresholds: {
        // Relaxed — we're observing behavior, not enforcing SLOs
        http_req_failed: ['rate<0.20'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
    const payload = JSON.stringify(generateTransaction());

    const res = http.post(`${BASE_URL}/v1/transactions/assess`, payload, {
        headers: { 'Content-Type': 'application/json' },
        timeout: '5s',
    });

    check(res, {
        'status is 2xx': (r) => r.status >= 200 && r.status < 300,
        'not 503 (backpressure)': (r) => r.status !== 503,
    });

    sleep(0.05);
}
