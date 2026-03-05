import { describe, it, expect } from 'vitest';
import { extractTryCloudflareUrl } from '../src/tunnel';

describe('extractTryCloudflareUrl', () => {
  it('extracts a URL from cloudflared log output', () => {
    const line = 'INF Your quick Tunnel has been created! Visit it at https://abc-123.trycloudflare.com';
    expect(extractTryCloudflareUrl(line)).toBe('https://abc-123.trycloudflare.com');
  });

  it('returns null when no URL is present', () => {
    expect(extractTryCloudflareUrl('just a normal log line')).toBeNull();
  });
});
