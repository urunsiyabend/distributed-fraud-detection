import http from 'k6/http';
import { check, sleep } from 'k6';
import { generateTransaction } from './data.js';

export const options = {
    stages: [
        { duration: '30s', target: 100 },  // ramp up
        { duration: '1m',  target: 100 },  // sustain
        { duration: '30s', target: 0 },    // ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<50'],
        http_req_failed: ['rate<0.05'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
    const payload = JSON.stringify(generateTransaction());

    const res = http.post(`${BASE_URL}/v1/transactions/assess`, payload, {
        headers: { 'Content-Type': 'application/json' },
    });

    check(res, {
        'status is 202': (r) => r.status === 202,
        'response has decision': (r) => {
            try {
                return JSON.parse(r.body).decision !== undefined;
            } catch (e) {
                return false;
            }
        },
    });

    sleep(0.1);
}
