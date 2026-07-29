#!/usr/bin/env node
// reqit CLI runner: Discovered API
// Generated at 2026-07-29T09:37:32-04:00

const https = require('https');
const http = require('http');

// GET / - GET {{BASE_URL}}/
async function run_a2386db5_aa33_41f1_82f1_61867d42a674() {
  const url = new URL("{{BASE_URL}}/");
  return new Promise((resolve, reject) => {
    const options = { method: "GET", hostname: url.hostname, port: url.port, path: url.pathname + url.search };
    const proto = url.protocol === 'https:' ? https : http;
    const req = proto.request(options, (resp) => {
      let data = '';
      resp.on('data', (chunk) => data += chunk);
      resp.on('end', () => resolve({ status: resp.statusCode, headers: resp.headers, body: data }));
    });
    req.on('error', reject);
    req.end();
  });
}

// POST /payments - POST {{BASE_URL}}/payments
async function run__0d88a0b9_562a_43c1_93a0_2496034e962b() {
  const url = new URL("{{BASE_URL}}/payments");
  return new Promise((resolve, reject) => {
    const options = { method: "POST", hostname: url.hostname, port: url.port, path: url.pathname + url.search };
    const proto = url.protocol === 'https:' ? https : http;
    const req = proto.request(options, (resp) => {
      let data = '';
      resp.on('data', (chunk) => data += chunk);
      resp.on('end', () => resolve({ status: resp.statusCode, headers: resp.headers, body: data }));
    });
    req.on('error', reject);
    req.write("{}");
    req.end();
  });
}

// GET /status - GET {{BASE_URL}}/status
async function run__5688d5a6_1269_441f_b44a_6f3c86430671() {
  const url = new URL("{{BASE_URL}}/status");
  return new Promise((resolve, reject) => {
    const options = { method: "GET", hostname: url.hostname, port: url.port, path: url.pathname + url.search };
    const proto = url.protocol === 'https:' ? https : http;
    const req = proto.request(options, (resp) => {
      let data = '';
      resp.on('data', (chunk) => data += chunk);
      resp.on('end', () => resolve({ status: resp.statusCode, headers: resp.headers, body: data }));
    });
    req.on('error', reject);
    req.end();
  });
}

// POST /users - POST {{BASE_URL}}/users
async function run__9fa7f937_4a17_4614_8026_4a07ee4a2472() {
  const url = new URL("{{BASE_URL}}/users");
  return new Promise((resolve, reject) => {
    const options = { method: "POST", hostname: url.hostname, port: url.port, path: url.pathname + url.search };
    const proto = url.protocol === 'https:' ? https : http;
    const req = proto.request(options, (resp) => {
      let data = '';
      resp.on('data', (chunk) => data += chunk);
      resp.on('end', () => resolve({ status: resp.statusCode, headers: resp.headers, body: data }));
    });
    req.on('error', reject);
    req.write("{}");
    req.end();
  });
}

// GET /users/{id} - GET {{BASE_URL}}/users/{id}
async function run__93598a1b_f814_4dbe_b8e9_fa05b5c9e880() {
  const url = new URL("{{BASE_URL}}/users/{id}");
  return new Promise((resolve, reject) => {
    const options = { method: "GET", hostname: url.hostname, port: url.port, path: url.pathname + url.search };
    const proto = url.protocol === 'https:' ? https : http;
    const req = proto.request(options, (resp) => {
      let data = '';
      resp.on('data', (chunk) => data += chunk);
      resp.on('end', () => resolve({ status: resp.statusCode, headers: resp.headers, body: data }));
    });
    req.on('error', reject);
    req.end();
  });
}

async function main() {
  const results = [];
  try { results.push(await run_a2386db5_aa33_41f1_82f1_61867d42a674()); } catch(e) { results.push({ error: e.message }); }
  try { results.push(await run__0d88a0b9_562a_43c1_93a0_2496034e962b()); } catch(e) { results.push({ error: e.message }); }
  try { results.push(await run__5688d5a6_1269_441f_b44a_6f3c86430671()); } catch(e) { results.push({ error: e.message }); }
  try { results.push(await run__9fa7f937_4a17_4614_8026_4a07ee4a2472()); } catch(e) { results.push({ error: e.message }); }
  try { results.push(await run__93598a1b_f814_4dbe_b8e9_fa05b5c9e880()); } catch(e) { results.push({ error: e.message }); }
  console.log(JSON.stringify(results, null, 2));
}

main().catch(console.error);
