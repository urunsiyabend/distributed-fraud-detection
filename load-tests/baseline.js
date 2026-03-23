import http from 'k6/http';
import { check } from 'k6';
import { generateTransaction } from './data.js';

export const options = {
    vus: 10,
    duration: '30s',
    thresholds: {
        http_req_duration: ['p(95)<20', 'p(99)<50'],
        http_req_failed: ['rate<0.01'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
    const payload = JSON.stringify(generateTransaction());

    const res = http.post(`${BASE_URL}/v1/transactions/assess`, payload, {
        headers: {
            'Content-Type': 'application/json',
            'X-Idempotency-Key': `idem-${Date.now()}-${Math.random()}`,
        },
    });

    check(res, {
        'status is 202': (r) => r.status === 202,
        'has transaction_id': (r) => JSON.parse(r.body).transaction_id !== '',
        'has decision': (r) => ['approved', 'blocked', 'review', 'pending'].includes(JSON.parse(r.body).decision),
        'fast_path is true or pending': (r) => {
            const body = JSON.parse(r.body);
            return body.fast_path === true || body.decision === 'pending';
        },
    });
}
