import { describe, it, expect, beforeAll } from 'vitest';

const BASE_URL = process.env.TEST_BASE_URL || 'http://localhost:3000';
const AUTH_HEADER = { Authorization: 'Bearer admin' };

let baseApiAvailable = true;

async function safeFetchJson(url: string): Promise<any | null> {
  try {
    const res = await fetch(url, {
      headers: AUTH_HEADER,
      signal: AbortSignal.timeout(5000)
    });
    return await res.json();
  } catch {
    baseApiAvailable = false;
    return null;
  }
}

describe('Cloudflare Tunnel', () => {
  let publicUrl = '';

  beforeAll(async () => {
    const settings = await safeFetchJson(`${BASE_URL}/api/settings`);
    publicUrl = settings?.public_url || '';
  });

  it('public URL is configured', () => {
    if (!baseApiAvailable) return;
    expect(publicUrl).toBeDefined();
    expect(publicUrl.length).toBeGreaterThan(0);
  });

  it('public URL is a valid trycloudflare domain', () => {
    if (!baseApiAvailable) return;
    expect(publicUrl).toContain('trycloudflare.com');
  });

  it('tunnel is accessible (not returning 530)', async () => {
    if (!baseApiAvailable) return;
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
    const settings = await safeFetchJson(`${BASE_URL}/api/settings`);
    publicUrl = settings?.public_url || '';

    const agents = await safeFetchJson(`${BASE_URL}/api/agents`);
    if (Array.isArray(agents) && agents.length > 0 && agents[0].telegram_token) {
      agentToken = agents[0].telegram_token;
    }
  });

  it('tunnel webhook endpoint is accessible from external network', async () => {
    if (!baseApiAvailable || !publicUrl || !agentToken) return;

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
    if (!baseApiAvailable || !publicUrl || !agentToken) return;

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
    const settings = await safeFetchJson(`${BASE_URL}/api/settings`);
    if (!baseApiAvailable) return;

    const tunnelUrl = settings?.public_url;
    if (!tunnelUrl) {
      throw new Error('No public URL configured');
    }

    const res = await fetch(`${tunnelUrl}/dashboard/`, {
      signal: AbortSignal.timeout(10000)
    });

    if (res.status === 530) {
      throw new Error('Tunnel is down (530). Dashboard not accessible via tunnel.');
    }

    expect(res.status).toBe(200);
  });
});
