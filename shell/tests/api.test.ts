import { describe, it, expect, beforeAll, afterAll } from 'vitest';

const BASE_URL = process.env.TEST_BASE_URL || 'http://localhost:3000';
const AUTH_HEADER = { 'Authorization': 'Bearer admin' };

describe('Health Checks', () => {
  it('server is running', async () => {
    const res = await fetch(`${BASE_URL}/api/settings`, { 
      headers: AUTH_HEADER 
    });
    expect(res.status).toBe(200);
  });

  it('docker is available', async () => {
    const res = await fetch(`${BASE_URL}/api/docker/status`, {
      headers: AUTH_HEADER
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data).toHaveProperty('ok');
  });
});

describe('Tunnel & Webhooks', () => {
  it('public URL is configured', async () => {
    const res = await fetch(`${BASE_URL}/api/settings`, {
      headers: AUTH_HEADER
    });
    const data = await res.json();
    expect(data.public_url).toBeDefined();
    expect(data.public_url).toContain('trycloudflare.com');
  });

  it('webhook sync endpoint works', async () => {
    const res = await fetch(`${BASE_URL}/api/webhooks/sync`, {
      method: 'POST',
      headers: AUTH_HEADER
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data).toHaveProperty('success');
    expect(data).toHaveProperty('count');
  });
});

describe('Agents API', () => {
  it('can list agents', async () => {
    const res = await fetch(`${BASE_URL}/api/agents`, {
      headers: AUTH_HEADER
    });
    expect(res.status).toBe(200);
    const agents = await res.json();
    expect(Array.isArray(agents)).toBe(true);
  });
});
