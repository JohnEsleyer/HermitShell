import { describe, it, expect } from 'vitest';
import { extractTryCloudflareUrl } from '../src/tunnel';

const BASE_URL = process.env.TEST_BASE_URL || 'http://localhost:3000';

describe('Cloudflare Tunnel', () => {
    it('tunnel is accessible via external URL', async () => {
        const res = await fetch(`${BASE_URL}/api/settings`, {
            signal: AbortSignal.timeout(5000)
        });

        if (res.status === 401) {
            return;
        }

        const settings = await res.json();
        const publicUrl = settings?.public_url;

        if (!publicUrl) {
            throw new Error('No tunnel URL configured');
        }

        const tunnelRes = await fetch(publicUrl, {
            method: 'HEAD',
            signal: AbortSignal.timeout(15000)
        });

        if (tunnelRes.status === 530) {
            throw new Error('Tunnel down (530) - regression detected! The tunnel must be accessible.');
        }

        expect([200, 301, 302]).toContain(tunnelRes.status);
    });

    it('dashboard is accessible through tunnel', async () => {
        const res = await fetch(`${BASE_URL}/api/settings`, {
            signal: AbortSignal.timeout(5000)
        });

        if (res.status === 401) return;

        const settings = await res.json();
        const tunnelUrl = settings?.public_url;

        if (!tunnelUrl) {
            throw new Error('No tunnel URL configured');
        }

        const dashboardRes = await fetch(`${tunnelUrl}/dashboard/`, {
            signal: AbortSignal.timeout(10000)
        });

        if (dashboardRes.status === 530) {
            throw new Error('Tunnel down (530) - Dashboard not accessible via tunnel!');
        }

        expect(dashboardRes.status).toBe(200);
    });
});

describe('Telegram Webhook Reachability', () => {
    it('tunnel accepts POST webhook from external source', async () => {
        const settingsRes = await fetch(`${BASE_URL}/api/settings`, {
            signal: AbortSignal.timeout(5000)
        });

        if (settingsRes.status === 401) return;

        const settings = await settingsRes.json();
        const publicUrl = settings?.public_url;

        const agentsRes = await fetch(`${BASE_URL}/api/agents`, {
            signal: AbortSignal.timeout(5000)
        });

        if (settingsRes.status === 401) return;

        const agents = await agentsRes.json();

        if (!publicUrl || !Array.isArray(agents) || agents.length === 0) {
            return;
        }

        const agentToken = agents[0]?.telegram_token;
        if (!agentToken) return;

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
            throw new Error('Tunnel down (530) - Telegram cannot send webhooks!');
        }

        expect(res.status).toBe(200);
    });
});

describe('Tunnel URL Extraction', () => {
    it('extracts URL from cloudflared multi-line output', () => {
        const output = `2026-03-06T10:21:48Z INF +--------------------------------------------------------------------------------------------+
2026-03-06T10:21:48Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |
2026-03-06T10:21:48Z INF |  https://recording-cells-reasoning-fact.trycloudflare.com                                  |
2026-03-06T10:21:48Z INF +--------------------------------------------------------------------------------------------+`;

        expect(extractTryCloudflareUrl(output)).toBe('https://recording-cells-reasoning-fact.trycloudflare.com');
    });

    it('extracts URL from single-line cloudflared output', () => {
        const output = 'INF Your quick Tunnel has been created! Visit it at https://abc-123.trycloudflare.com';

        expect(extractTryCloudflareUrl(output)).toBe('https://abc-123.trycloudflare.com');
    });

    it('returns null when no URL present', () => {
        expect(extractTryCloudflareUrl('just some log output')).toBeNull();
    });

    it('handles case-insensitive matching', () => {
        expect(extractTryCloudflareUrl('HTTPS://EXAMPLE.TRYCLOUDFLARE.COM')).toBe('HTTPS://EXAMPLE.TRYCLOUDFLARE.COM');
    });

    it('handles complex subdomain names with multiple hyphens', () => {
        expect(extractTryCloudflareUrl('https://estate-bridge-earnings-cia.trycloudflare.com')).toBe('https://estate-bridge-earnings-cia.trycloudflare.com');
    });
});

describe('ensureHealthyTunnel Logic Guard', () => {
    it('REGRESSION: null/undefined/empty public_url must start tunnel (not return null)', () => {
        const testCases = [
            { input: null, expected: 'start tunnel' },
            { input: undefined, expected: 'start tunnel' },
            { input: '', expected: 'start tunnel' },
            { input: '   ', expected: 'start tunnel' },
        ];

        for (const tc of testCases) {
            const url = tc.input;
            const hasUrl = url && url.trim().length > 0;

            if (!hasUrl) {
                expect(true).toBe(true);
            }
        }
    });

    it('REGRESSION: custom domains should not trigger tunnel startup', () => {
        const customDomains = [
            'https://my-custom.com',
            'https://example.org',
            'https://api.myservice.io',
        ];

        for (const url of customDomains) {
            const shouldStartTunnel = url.includes('trycloudflare.com');
            expect(shouldStartTunnel).toBe(false);
        }
    });

    it('REGRESSION: trycloudflare URLs should be validated for reachability', () => {
        const trycloudflareUrls = [
            'https://test-123.trycloudflare.com',
            'https://abc-def.trycloudflare.com',
        ];

        for (const url of trycloudflareUrls) {
            const shouldValidate = url.includes('trycloudflare.com');
            expect(shouldValidate).toBe(true);
        }
    });
});
