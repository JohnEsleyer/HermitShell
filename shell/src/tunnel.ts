import { spawn, ChildProcess } from 'child_process';
import { promises as dns } from 'dns';
import { setSetting, getSetting } from './db';

let tunnelProcess: ChildProcess | null = null;
let currentUrl: string | null = null;
let restartAttempts = 0;
const MAX_RESTART_ATTEMPTS = 5;

/**
 * Extracts trycloudflare URL from cloudflared output.
 * Handles both single-line and multi-line output formats.
 * 
 * Single-line: "INF Your quick Tunnel has been created! Visit it at https://abc-123.trycloudflare.com"
 * Multi-line:  "+------------------------------------------------------------------+
 *                |  https://recording-cells-reasoning-fact.trycloudflare.com   |"
 */
export function extractTryCloudflareUrl(output: string): string | null {
    const match = output.match(/https:\/\/[a-z0-9-]+\.trycloudflare\.com/i);
    return match ? match[0] : null;
}

async function readTunnelUrlFromProcess(process: ChildProcess): Promise<string | null> {
    return new Promise((resolve) => {
        const onData = (data: Buffer) => {
            const line = data.toString();
            const url = extractTryCloudflareUrl(line);
            if (!url) return;
            cleanup();
            resolve(url);
        };

        const cleanup = () => {
            process.stdout?.off('data', onData);
            process.stderr?.off('data', onData);
        };

        process.stdout?.on('data', onData);
        process.stderr?.on('data', onData);
    });
}


async function waitForPublicHostname(url: string, attempts = 8, delayMs = 3000): Promise<boolean> {
    try {
        const hostname = new URL(url).hostname;

        for (let i = 0; i < attempts; i++) {
            try {
                const records = await dns.resolve4(hostname);
                if (records.length > 0) return true;
            } catch {
                // DNS may still be propagating for fresh Quick Tunnel hostnames
            }

            if (i < attempts - 1) {
                await new Promise((resolve) => setTimeout(resolve, delayMs));
            }
        }
    } catch {
        return false;
    }

    return false;
}

async function registerWebhookWithRetry(registerFn: () => Promise<boolean>, attempts = 4, delayMs = 4000): Promise<boolean> {
    for (let i = 0; i < attempts; i++) {
        const ok = await registerFn();
        if (ok) return true;

        if (i < attempts - 1) {
            await new Promise((resolve) => setTimeout(resolve, delayMs * (i + 1)));
        }
    }

    return false;
}
async function isTunnelEndpointReachable(url: string): Promise<boolean> {
    try {
        const res = await fetch(`${url}/dashboard/`, {
            method: 'HEAD',
            signal: AbortSignal.timeout(10000)
        });
        return res.status !== 530;
    } catch {
        return false;
    }
}

export function getTunnelUrl(): string | null {
    return currentUrl;
}

export function isTunnelRunning(): boolean {
    return tunnelProcess !== null && !tunnelProcess.killed;
}

/**
 * Starts a new Cloudflare Quick Tunnel.
 * 
 * IMPORTANT: This function MUST return a valid URL string, never null,
 * when cloudflared successfully starts. The caller (ensureHealthyTunnel)
 * relies on this behavior to determine if tunnel startup succeeded.
 * 
 * @param port Local port to tunnel
 * @returns The public trycloudflare URL, or null only if tunnel failed to start
 */
export async function startTunnel(port: number): Promise<string | null> {
    if (tunnelProcess && !tunnelProcess.killed) {
        console.log('[Tunnel] Already running, returning existing URL');
        return currentUrl;
    }

    // Force fresh tunnel every time (bypass cache)
    // const existingUrl = await getSetting('public_url');
    // if (existingUrl && existingUrl.includes('trycloudflare.com')) {
    //     const recent = await getSetting('tunnel_started_at');
    //     if (recent) {
    //         const startedAt = new Date(recent).getTime();
    //         const hoursSinceStart = (Date.now() - startedAt) / (1000 * 60 * 60);
    //         if (hoursSinceStart < 12) {
    //             console.log('[Tunnel] Using existing tunnel URL from DB');
    //             currentUrl = existingUrl;
    //             return existingUrl;
    //         }
    //     }
    // }

    return new Promise((resolve) => {
        console.log('[Tunnel] Starting Cloudflare Quick Tunnel...');

        try {
            tunnelProcess = spawn('cloudflared', [
                'tunnel',
                '--url', `http://localhost:${port}`
            ], {
                stdio: ['ignore', 'pipe', 'pipe']
            });

            const urlPromise = readTunnelUrlFromProcess(tunnelProcess);

            let resolved = false;
            const timeout = setTimeout(() => {
                if (!resolved) {
                    resolved = true;
                    console.log('[Tunnel] Timeout waiting for URL, continuing without tunnel');
                    resolve(null);
                }
            }, 30000);

            urlPromise.then(async (url) => {
                if (url && !resolved) {
                    clearTimeout(timeout);
                    resolved = true;
                    currentUrl = url;

                    console.log(`[Tunnel] ✅ Public URL: ${currentUrl}`);

                    await setSetting('public_url', currentUrl);
                    await setSetting('tunnel_started_at', new Date().toISOString());

                    restartAttempts = 0;
                    resolve(currentUrl);
                }
            });

            tunnelProcess.on('error', (err) => {
                console.error('[Tunnel] Failed to start:', err.message);
                if (!resolved) {
                    clearTimeout(timeout);
                    resolved = true;
                    resolve(null);
                }
            });

            tunnelProcess.on('exit', (code) => {
                console.log(`[Tunnel] Process exited with code ${code}`);
                tunnelProcess = null;

                if (!resolved) {
                    clearTimeout(timeout);
                    resolved = true;
                    resolve(null);
                } else if (code !== 0 && restartAttempts < MAX_RESTART_ATTEMPTS) {
                    restartAttempts++;
                    console.log(`[Tunnel] Restarting (attempt ${restartAttempts}/${MAX_RESTART_ATTEMPTS})...`);
                    setTimeout(() => startTunnel(port), 5000);
                }
            });
        } catch (err: any) {
            console.error('[Tunnel] Spawn error:', err.message);
            resolve(null);
        }
    });
}

/**
 * Ensures a healthy tunnel is running.
 * 
 * CRITICAL BEHAVIOR - DO NOT MODIFY WITHOUT UNDERSTANDING:
 * 1. If public_url is null/empty/undefined: MUST start a NEW tunnel (never return null!)
 * 2. If public_url is a custom domain (doesn't contain trycloudflare.com): Return it as-is
 * 3. If public_url is a trycloudflare URL: Check if reachable
 *    - If reachable: Use existing URL
 *    - If NOT reachable (530): Clear URL, stop old tunnel, start NEW tunnel
 * 
 * REGRESSION WARNING (2026-03-06):
 * A previous bug caused this function to return null when public_url was missing from DB,
 * which resulted in DNS_PROBE_FINISHED_NXDOMAIN errors. The fix ensures startTunnel()
 * is ALWAYS called when no valid public_url exists.
 */
export async function ensureHealthyTunnel(port: number): Promise<string | null> {
    const existingUrl = await getSetting('public_url');
    
    // CASE 1: No URL in database - MUST start new tunnel
    // This was the bug: previously returned null here instead of calling startTunnel()
    if (!existingUrl || existingUrl.trim() === '') {
        console.log('[Tunnel] No public_url found, starting new tunnel...');
        return startTunnel(port);
    }
    
    // CASE 2: Custom domain - use as-is (not a trycloudflare URL)
    if (!existingUrl.includes('trycloudflare.com')) {
        return existingUrl;
    }

    // CASE 3: trycloudflare URL - verify it's still working
    const reachable = await isTunnelEndpointReachable(existingUrl);
    if (reachable) {
        currentUrl = existingUrl;
        return existingUrl;
    }

    // CASE 4: trycloudflare URL is broken (530) - restart tunnel
    console.log('[Tunnel] Existing trycloudflare URL is unhealthy, restarting tunnel...');
    await setSetting('public_url', '');
    stopTunnel();
    return startTunnel(port);
}

export function stopTunnel(): void {
    if (tunnelProcess && !tunnelProcess.killed) {
        console.log('[Tunnel] Stopping...');
        tunnelProcess.kill('SIGTERM');
        tunnelProcess = null;
        currentUrl = null;
    }
}

let healthCheckInterval: NodeJS.Timeout | null = null;
let lastHealthCheckStatus = true;

export function startTunnelHealthCheck(port: number, intervalMs = 60000): void {
    if (healthCheckInterval) {
        console.log('[Tunnel] Health check already running');
        return;
    }

    console.log(`[Tunnel] Starting health check (every ${intervalMs}ms)...`);

    healthCheckInterval = setInterval(async () => {
        const url = currentUrl;
        if (!url) {
            console.log('[Tunnel] Health check: No URL, starting new tunnel...');
            lastHealthCheckStatus = false;
            await ensureHealthyTunnel(port);
            return;
        }

        if (!url.includes('trycloudflare.com')) {
            return;
        }

        const reachable = await isTunnelEndpointReachable(url);
        if (!reachable) {
            console.log('[Tunnel] Health check: Tunnel unhealthy (530), restarting...');
            await setSetting('public_url', '');
            stopTunnel();
            await startTunnel(port);
            lastHealthCheckStatus = false;
        } else {
            lastHealthCheckStatus = true;
        }
    }, intervalMs);
}

export function stopTunnelHealthCheck(): void {
    if (healthCheckInterval) {
        clearInterval(healthCheckInterval);
        healthCheckInterval = null;
        console.log('[Tunnel] Health check stopped');
    }
}

export async function syncWebhooks(port: number): Promise<number> {
    const { getAllAgents } = await import('./db');
    const { registerWebhook } = await import('./telegram');

    const WEBHOOK_SECRET = process.env.WEBHOOK_SECRET || 'hermitshell-webhook-secret';

    if (!currentUrl) {
        console.log('[Tunnel] No tunnel URL, skipping webhook sync');
        return 0;
    }

    const agents = await getAllAgents();
    let successCount = 0;

    const hostnameReady = await waitForPublicHostname(currentUrl);
    if (!hostnameReady) {
        console.warn('[Tunnel] Tunnel hostname is not resolvable yet; webhook sync may fail until DNS propagates.');
    }

    for (const agent of agents) {
        if (agent.is_active && agent.telegram_token) {
            try {
                const ok = await registerWebhookWithRetry(
                    () => registerWebhook(agent.telegram_token, currentUrl!, WEBHOOK_SECRET)
                );
                if (ok) successCount++;
            } catch (err) {
                console.error(`[Tunnel] Failed to sync webhook for agent ${agent.id}`);
            }
        }
    }

    console.log(`[Tunnel] Synced ${successCount}/${agents.length} webhooks`);
    return successCount;
}
