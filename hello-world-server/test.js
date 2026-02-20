const http = require('http');
const assert = require('assert');

const port = 3000;
const url = `http://localhost:${port}/`;

// Helper to wait for the server to be ready
function waitForServer(retries = 5) {
  return new Promise((resolve, reject) => {
    const check = () => {
      http.get(url, (res) => {
        resolve();
      }).on('error', (err) => {
        if (retries > 0) {
          setTimeout(() => {
            retries--;
            check();
          }, 100);
        } else {
          reject(new Error('Server not ready'));
        }
      });
    };
    check();
  });
}

async function test() {
  console.log('Starting test...');
  
  // Wait for server (assuming it's already started or starting)
  try {
    await waitForServer();
    
    http.get(url, (res) => {
      assert.strictEqual(res.statusCode, 200);
      
      let data = '';
      res.on('data', (chunk) => {
        data += chunk;
      });
      
      res.on('end', () => {
        assert.strictEqual(data, 'Hello, World!\n');
        console.log('Test passed!');
        process.exit(0);
      });
    }).on('error', (err) => {
      console.error('Test failed:', err.message);
      process.exit(1);
    });
  } catch (err) {
    console.error('Test failed:', err.message);
    process.exit(1);
  }
}

test();
