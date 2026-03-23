import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';
import { generateTransaction } from './data.js';

const goroutines = new Trend('go_goroutines');
const memAlloc = new Trend('go_mem_alloc_bytes');

export const options = {
    vus: 50,
    duration: '10m',
    thresholds: {
        http_req_duration: ['p(95)<50'],
        http_req_failed: ['rate<0.01'],
    },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const METRICS_URL = __ENV.METRICS_URL || 'http://localhost:8080/metrics';

export default function () {
    const payload = JSON.stringify(generateTransaction());

    const res = http.post(`${BASE_URL}/v1/transactions/assess`, payload, {
        headers: { 'Content-Type': 'application/json' },
    });

    check(res, {
        'status is 202': (r) => r.status === 202,
    });

    // Every ~100 iterations, scrape Go runtime metrics
    if (Math.random() < 0.01) {
        const metricsRes = http.get(METRICS_URL);
        if (metricsRes.status === 200) {
            const body = metricsRes.body;

            const goroutineMatch = body.match(/go_goroutines\s+(\d+)/);
            if (goroutineMatch) {
                goroutines.add(parseInt(goroutineMatch[1]));
            }

            const memMatch = body.match(/go_memstats_alloc_bytes\s+(\d+)/);
            if (memMatch) {
                memAlloc.add(parseInt(memMatch[1]));
            }
        }
    }

    sleep(0.1);
}

export function handleSummary(data) {
    const g = data.metrics.go_goroutines;
    const m = data.metrics.go_mem_alloc_bytes;

    console.log('=== Soak Test Runtime Metrics ===');
    if (g) {
        console.log(`Goroutines — min: ${g.values.min}, max: ${g.values.max}, avg: ${g.values.avg.toFixed(0)}`);
    }
    if (m) {
        const toMB = (b) => (b / 1024 / 1024).toFixed(1);
        console.log(`Memory alloc — min: ${toMB(m.values.min)}MB, max: ${toMB(m.values.max)}MB, avg: ${toMB(m.values.avg)}MB`);
    }

    return {};
}
