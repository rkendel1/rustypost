// Auto-generated Playwright test from reqit collection: Discovered API
// Generated at 2026-07-29T09:37:32-04:00
import const { test, expect } = require('@playwright/test');


test('GET /', async ({ request }) => {
  const resp = await request.get("{{BASE_URL}}/");
  
    expect(resp.status()).toBe(200);
  
});

test('POST /payments', async ({ request }) => {
  const resp = await request.post("{{BASE_URL}}/payments", {
    data: {},
    headers: {}
  });
  
    expect(resp.status()).toBe(200);
  
});

test('GET /status', async ({ request }) => {
  const resp = await request.get("{{BASE_URL}}/status");
  
    expect(resp.status()).toBe(200);
  
});

test('POST /users', async ({ request }) => {
  const resp = await request.post("{{BASE_URL}}/users", {
    data: {},
    headers: {}
  });
  
    expect(resp.status()).toBe(200);
  
});

test('GET /users/{id}', async ({ request }) => {
  const resp = await request.get("{{BASE_URL}}/users/{id}");
  
    expect(resp.status()).toBe(200);
  
});

