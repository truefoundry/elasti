import requests
import time
from tqdm import tqdm


successful = 0
failed = 0

for i in range(60):
    response = requests.get('https://ishaan-new-test-1-ishaan-ws-80.tfy-ctl-euwe1-devtest.devtest.truefoundry.tech/')
    print(f"Attempt {i+1}: {response.status_code}")
    print(f"Response time: {response.elapsed.total_seconds()}")
    if response.status_code == 200:
        successful += 1
    else:
        failed += 1
    for j in tqdm(range(80), desc="Sleeping"):
        time.sleep(1)
