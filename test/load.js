import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';
import { SharedArray } from 'k6/data';

export const options = {
        stages: [
                { duration: '10s', target: 1000},
                { duration: '10s', target: 1000},
                { duration: '10s', target: 0},
        ],
}

const failureRate = [];
failureRate[0] = new Rate('failed_requests_1');
failureRate[1] = new Rate('failed_requests_2');
failureRate[2] = new Rate('failed_requests_3');

const load_url = "https://httpbin-hello-raman-ws-80.zurichlabs-cluster.truefoundry.tfy.app/headers"
const load_url_1 = "https://httpbin-hello-1-raman-ws-80.zurichlabs-cluster.truefoundry.tfy.app/headers"
const load_url_2 = "https://httpbin-hello-2-raman-ws-80.zurichlabs-cluster.truefoundry.tfy.app/headers"
const params = {
    headers: {
      "Content-Type": "application/json"
    }
};

export default function () {
        rpc(load_url, 0);
        rpc(load_url_1, 1);
        rpc(load_url_2, 2);
} 

function rpc(load_url, index){
        const res = http.get(load_url, params);
        const checkResult = check(res, {'status was 200': (r) => r.status == 200})
        failureRate[index].add(!checkResult);
}
