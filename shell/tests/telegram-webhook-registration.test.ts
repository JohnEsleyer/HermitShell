import { afterEach, describe, expect, it, vi } from 'vitest';
import { registerWebhook } from '../src/telegram';

describe('registerWebhook', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it('retries when Telegram reports unresolved webhook hostname and eventually succeeds', async () => {
    vi.useFakeTimers();

    let setWebhookAttempts = 0;
    const fetchMock = vi.fn(async (url: string) => {
      if (url.includes('/deleteWebhook')) {
        return { json: async () => ({ ok: true }) } as any;
      }

      if (url.includes('/setWebhook')) {
        setWebhookAttempts += 1;
        if (setWebhookAttempts < 3) {
          return {
            json: async () => ({
              ok: false,
              description: 'Bad Request: bad webhook: Failed to resolve host: Name or service not known'
            })
          } as any;
        }

        return { json: async () => ({ ok: true }) } as any;
      }

      if (url.includes('/setMyCommands')) {
        return { json: async () => ({ ok: true }) } as any;
      }

      throw new Error(`Unexpected URL: ${url}`);
    });

    vi.stubGlobal('fetch', fetchMock);

    const resultPromise = registerWebhook('token123', 'https://test-subdomain.trycloudflare.com', 'secret123');

    await vi.runAllTimersAsync();

    await expect(resultPromise).resolves.toBe(true);
    expect(setWebhookAttempts).toBe(3);
  });

  it('does not retry on non-DNS webhook errors', async () => {
    const fetchMock = vi.fn(async (url: string) => {
      if (url.includes('/deleteWebhook')) {
        return { json: async () => ({ ok: true }) } as any;
      }

      if (url.includes('/setWebhook')) {
        return {
          json: async () => ({
            ok: false,
            description: 'Bad Request: bad webhook: HTTPS URL must be provided for webhook'
          })
        } as any;
      }

      throw new Error(`Unexpected URL: ${url}`);
    });

    vi.stubGlobal('fetch', fetchMock);

    await expect(registerWebhook('token123', 'https://example.com', 'secret123')).resolves.toBe(false);

    const setWebhookCalls = fetchMock.mock.calls.filter(([url]) => String(url).includes('/setWebhook'));
    expect(setWebhookCalls).toHaveLength(1);
  });
});
