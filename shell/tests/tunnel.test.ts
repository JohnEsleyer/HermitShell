import { describe, it, expect, beforeAll } from 'vitest';

const BASE_URL = process.env.TEST_BASE_URL || 'http://localhost:3000';
const AUTH_HEADER = { 'Authorization': 'Bearer admin' };

describe('Cloudflare Tunnel', () => {
  let publicUrl = '';

  beforeAll(async () => {
    const res = await fetch(`${BASE_URL}/api/settings`, { headers: AUTH_HEADER });
    const settings = await res.json();
    publicUrl = settings.public_url || '';
  });

  it('public URL is configured', () => {
    expect(publicUrl).toBeDefined();
    expect(publicUrl.length).toBeGreaterThan(0);
  });

  it('public URL is a valid trycloudflare domain', () => {
    expect(publicUrl).toContain('trycloudflare.com');
  });

  it('tunnel is accessible (not returning 530)', async () => {
    if (!publicUrl) {
      throw new Error('No public URL configured');
    }
    
    const res = await fetch(publicUrl, { 
      method: 'HEAD',
      signal: AbortSignal.timeout(10000)
    });
    
    if (res.status === 530) {
      throw new Error('Tunnel is down (530). Cloudflare tunnel needs restart.');
    }
    
    expect([200, 301, 302, 303, 307, 308]).toContain(res.status);
  });
});

describe('Telegram Webhook Reachability', () => {
  let publicUrl = '';
  let agentToken = '';

  beforeAll(async () => {
    const settingsRes = await fetch(`${BASE_URL}/api/settings`, { headers: AUTH_HEADER });
    const settings = await settingsRes.json();
    publicUrl = settings.public_url || '';

    const agentsRes = await fetch(`${BASE_URL}/api/agents`, { headers: AUTH_HEADER });
    const agents = await agentsRes.json();
    if (agents.length > 0 && agents[0].telegram_token) {
      agentToken = agents[0].telegram_token;
    }
  });

  it('tunnel webhook endpoint is accessible from external network', async () => {
    if (!publicUrl || !agentToken) {
      console.log('Skipping - missing public URL or agent token');
      return;
    }

    const webhookUrl = `${publicUrl}/webhook/${agentToken}?secret=test`;
    const res = await fetch(webhookUrl, {
      method: 'GET',
      signal: AbortSignal.timeout(15000)
    });

    if (res.status === 530) {
      throw new Error('Tunnel is down (530). Telegram cannot reach webhook. Restart cloudflared tunnel.');
    }

    expect(res.status).toBe(200);
  });

  it('tunnel accepts POST webhook from external source', async () => {
    if (!publicUrl || !agentToken) {
      console.log('Skipping - missing public URL or agent token');
      return;
    }

    const WEBHOOK_SECRET = process.env.WEBHOOK_SECRET || 'hermitshell-webhook-secret';
    const cleanSecret = WEBHOOK_SECRET.replace(/[^a-zA-Z0-9_-]/g, '') || 'hermitSecret123';
    
    const webhookUrl = `${publicUrl}/webhook/${agentToken}?secret=${cleanSecret}`;
    
    const res = await fetch(webhookUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        message: {
          text: '/start',
          chat: { id: 999999999 },
          from: { id: 999999999, first_name: 'TestBot' }
        }
      }),
      signal: AbortSignal.timeout(15000)
    });

    if (res.status === 530) {
      throw new Error('Tunnel is down (530). Telegram cannot send webhooks. Restart cloudflared tunnel.');
    }

    expect(res.status).toBe(200);
    const data = await res.json();
    expect(data.ok).toBe(true);
  });
});

describe('Dashboard via Tunnel', () => {
  it('dashboard is accessible through tunnel', async () => {
    const res = await fetch(`${BASE_URL}/api/settings`, { headers: AUTH_HEADER });
    const settings = await res.json();
    const tunnelUrl = settings.public_url;
    
    if (!tunnelUrl) {
      throw new Error('No public URL configured');
    }
    
    const res2 = await fetch(`${tunnelUrl}/dashboard/`, {
      signal: AbortSignal.timeout(10000)
    });

    if (res2.status === 530) {
      throw new Error('Tunnel is down (530). Dashboard not accessible via tunnel.');
    }
    
    expect(res2.status).toBe(200);
  });
});
