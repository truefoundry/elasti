# Load testing

## 1. Update k6 tests

   Update `./test/load.js` to set your target URL and adjust any other configuration values.

## 2. Run load.js

   Run the following command to run the test.

   ```bash
   chmod +x ./test/generate_load.sh
   cd ./test
   ./generate_load.sh
   ```