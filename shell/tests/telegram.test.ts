import { describe, it, expect, beforeAll } from 'vitest';

const BASE_URL = process.env.TEST_BASE_URL || 'http://localhost:3000';
const AUTH_HEADER = { 'Authorization': 'Bearer admin' };

describe('Telegram Webhook', () => {
  let agentToken = '';
  let publicUrl = '';

  beforeAll(async () => {
    const res = await fetch(`${BASE_URL}/api/settings`, { headers: AUTH_HEADER });
    const settings = await res.json();
    publicUrl = settings.public_url || '';

    const agentsRes = await fetch(`${BASE_URL}/api/agents`, { headers: AUTH_HEADER });
    const agents = await agentsRes.json();
    if (agents.length > 0 && agents[0].telegram_token) {
      agentToken = agents[0].telegram_token;
    }
  });

  it('webhook endpoint responds to GET', async () => {
    if (!agentToken) return;
    const res = await fetch(`${BASE_URL}/webhook/${agentToken}`);
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.message).toContain('POST');
  });

  it('webhook validates secret token', async () => {
    if (!agentToken) return;
    const res = await fetch(`${BASE_URL}/webhook/${agentToken}?secret=wrongsecret`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message: { text: 'test', chat: { id: 123 }, from: { id: 456 } } })
    });
    expect(res.status).toBe(403);
  });

  it('webhook accepts valid update', async () => {
    if (!agentToken) return;
    const WEBHOOK_SECRET = process.env.WEBHOOK_SECRET || 'hermitshell-webhook-secret';
    const cleanSecret = WEBHOOK_SECRET.replace(/[^a-zA-Z0-9_-]/g, '') || 'hermitSecret123';
    
    const res = await fetch(`${BASE_URL}/webhook/${agentToken}?secret=${cleanSecret}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        message: {
          text: '/start',
          chat: { id: 123456 },
          from: { id: 123456, first_name: 'Test' }
        }
      })
    });
    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.ok).toBe(true);
  });
});

describe('Telegram Bot Commands', () => {
  it('can trigger webhook reset for agent', async () => {
    const agentsRes = await fetch(`${BASE_URL}/api/agents`, { headers: AUTH_HEADER });
    const agents = await agentsRes.json();
    
    if (agents.length > 0) {
      const res = await fetch(`${BASE_URL}/api/webhooks/reset/${agents[0].id}`, {
        method: 'POST',
        headers: AUTH_HEADER
      });
      expect(res.status).toBe(200);
      const data = await res.json();
      expect(data).toHaveProperty('success');
    }
  });
});
