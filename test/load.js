import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
        stages: [
                { duration: '15s', target: 200},
                { duration: '15s', target: 500},
        ],
}

export default function () {
        const load_url = "https://emotion-class-svc-raman-ws-8000.zurichlabs-cluster.truefoundry.tfy.app/predict"
        const payload = JSON.stringify({
            "inputs": [
              "I am happy",
              "I am angry",
              "I am sad"
            ],
            "parameters": {}
          });

          const params = {
            headers: {
              "Content-Type": "application/json"
            }
          };


        const res = http.post(load_url, payload, params);
        check(res, {'status was 200': (r) => r.status == 200})
}

