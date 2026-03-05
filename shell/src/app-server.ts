import * as fs from 'fs';
import * as path from 'path';
import { spawn, ChildProcess } from 'child_process';
import { discoverSitesFromWorkspaces } from './sites';
import { getAllAgents, getAllSettings, createAppServer, getActiveAppServer, deactivateAppServer, updateAppServerScreenshot, getAllActiveAppServers, deleteAppServer, createSiteTunnel, getActiveSiteTunnel, deactivateSiteTunnel } from './db';

const WORKSPACE_DIR = path.join(__dirname, '../../data/workspaces');

interface AppProcess {
    process: ChildProcess;
    port: number;
    agentId: number;
    userId: number;
    siteName: string;
}

const appProcesses = new Map<string, AppProcess>();
let portCounter = 9000;
const PROCESS_TIMEOUT = 30000;

function getProcessKey(agentId: number, userId: number, siteName: string): string {
    return `${agentId}_${userId}_${siteName}`;
}

export async function scanApps(): Promise<{ discovered: number; apps: any[] }> {
    const agents = await getAllAgents();
    const settings = await getAllSettings();
    const baseUrl = settings.public_url || 'http://localhost:3000';
    
    const siteRecords = discoverSitesFromWorkspaces(WORKSPACE_DIR, agents, baseUrl, () => null);
    
    const validApps: any[] = [];
    
    for (const site of siteRecords) {
        for (const app of site.webApps) {
            if (app.hasIndexHtml) {
                const activeServer = await getActiveAppServer(app.agentId, app.userId, app.siteName);
                validApps.push({
                    ...app,
                    isActive: !!activeServer,
                    port: activeServer?.port,
                    screenshotPath: activeServer?.screenshot_path
                });
            }
        }
    }
    
    return { discovered: validApps.length, apps: validApps };
}

export async function startAppServer(agentId: number, userId: number, siteName: string): Promise<{ success: boolean; port?: number; error?: string }> {
    const key = getProcessKey(agentId, userId, siteName);
    
    if (appProcesses.has(key)) {
        const existing = appProcesses.get(key)!;
        return { success: true, port: existing.port };
    }
    
    const workspacePath = path.join(WORKSPACE_DIR, `${agentId}_${userId}`);
    const appPath = path.join(workspacePath, 'www', siteName);
    
    if (!fs.existsSync(appPath) || !fs.statSync(appPath).isDirectory()) {
        return { success: false, error: 'App directory not found' };
    }
    
    const indexPath = path.join(appPath, 'index.html');
    if (!fs.existsSync(indexPath)) {
        return { success: false, error: 'index.html not found' };
    }
    
    const port = portCounter++;
    
    return new Promise((resolve) => {
        const serveProcess = spawn('python3', ['-m', 'http.server', String(port)], {
            cwd: appPath,
            stdio: ['ignore', 'pipe', 'pipe'],
            detached: false
        });
        
        let started = false;
        const timeout = setTimeout(() => {
            if (!started) {
                serveProcess.kill();
                resolve({ success: false, error: 'Timeout starting server' });
            }
        }, 10000);
        
        serveProcess.stderr?.on('data', (data: Buffer) => {
            const output = data.toString();
            if (output.includes('Serving HTTP') && !started) {
                started = true;
                clearTimeout(timeout);
                appProcesses.set(key, {
                    process: serveProcess,
                    port,
                    agentId,
                    userId,
                    siteName
                });
                
                createAppServer({
                    agent_id: agentId,
                    user_id: userId,
                    site_name: siteName,
                    port
                });
                
                resolve({ success: true, port });
            }
        });
        
        serveProcess.on('error', (err) => {
            clearTimeout(timeout);
            resolve({ success: false, error: err.message });
        });
        
        serveProcess.on('exit', (code) => {
            appProcesses.delete(key);
        });
    });
}

export async function stopAppServer(agentId: number, userId: number, siteName: string): Promise<{ success: boolean }> {
    const key = getProcessKey(agentId, userId, siteName);
    const appProcess = appProcesses.get(key);
    
    if (appProcess) {
        appProcess.process.kill('SIGTERM');
        appProcesses.delete(key);
    }
    
    await deactivateAppServer(agentId, userId, siteName);
    
    return { success: true };
}

export async function getAppServerStatus(agentId: number, userId: number, siteName: string): Promise<{ isActive: boolean; port?: number }> {
    const key = getProcessKey(agentId, userId, siteName);
    const appProcess = appProcesses.get(key);
    
    if (appProcess) {
        return { isActive: true, port: appProcess.port };
    }
    
    const activeServer = await getActiveAppServer(agentId, userId, siteName);
    return { isActive: !!activeServer, port: activeServer?.port ?? undefined };
}

export async function captureAppScreenshotInContainer(
    agentId: number,
    userId: number,
    siteName: string
): Promise<{ success: boolean; screenshotPath?: string; error?: string }> {
    const status = await getAppServerStatus(agentId, userId, siteName);
    
    if (!status.isActive || !status.port) {
        return { success: false, error: 'App server is not running' };
    }
    
    const workspacePath = path.join(WORKSPACE_DIR, `${agentId}_${userId}`);
    const appPath = path.join(workspacePath, 'www', siteName);
    const screenshotPath = path.join(appPath, 'thumbnail.png');
    
    const targetUrl = `http://127.0.0.1:${status.port}/`;
    
    let playwright: any;
    try {
        playwright = await import('playwright');
    } catch {
        return { success: false, error: 'Playwright is not installed' };
    }
    
    const browser = await playwright.chromium.launch({ headless: true });
    const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
    
    try {
        await page.goto(targetUrl, { waitUntil: 'networkidle', timeout: 15000 });
        await page.screenshot({ path: screenshotPath });
        await browser.close();
        
        await updateAppServerScreenshot(agentId, userId, siteName, screenshotPath);
        
        return { success: true, screenshotPath };
    } catch (err: any) {
        await browser.close();
        return { success: false, error: err.message };
    }
}

export async function createAppTunnel(
    agentId: number,
    userId: number,
    siteName: string
): Promise<{ success: boolean; tunnelUrl?: string; error?: string }> {
    const status = await getAppServerStatus(agentId, userId, siteName);
    
    if (!status.isActive || !status.port) {
        return { success: false, error: 'App server is not running' };
    }
    
    const existingTunnel = await getActiveSiteTunnel(agentId, userId, siteName);
    if (existingTunnel && existingTunnel.tunnel_url) {
        return { success: true, tunnelUrl: existingTunnel.tunnel_url };
    }
    
    const expiresAt = new Date(Date.now() + 30 * 60 * 1000).toISOString();
    
    return new Promise((resolve) => {
        const tunnelProcess = spawn('cloudflared', [
            'tunnel',
            '--url', `http://localhost:${status.port}`
        ], {
            stdio: ['ignore', 'pipe', 'pipe']
        });
        
        let resolved = false;
        const timeout = setTimeout(() => {
            if (!resolved) {
                resolved = true;
                tunnelProcess.kill();
                resolve({ success: false, error: 'Timeout creating tunnel' });
            }
        }, 30000);
        
        tunnelProcess.stderr?.on('data', async (data: Buffer) => {
            const line = data.toString();
            const match = line.match(/https:\/\/[a-z0-9-]+\.trycloudflare\.com/);
            
            if (match && !resolved) {
                clearTimeout(timeout);
                resolved = true;
                
                const tunnelUrl = match[0];
                
                await createSiteTunnel({
                    agent_id: agentId,
                    user_id: userId,
                    site_name: siteName,
                    tunnel_url: tunnelUrl,
                    expires_at: expiresAt
                });
                
                tunnelProcess.on('exit', async () => {
                    const tunnel = await getActiveSiteTunnel(agentId, userId, siteName);
                    if (tunnel) {
                        await deactivateSiteTunnel(tunnel.id);
                    }
                });
                
                resolve({ success: true, tunnelUrl });
            }
        });
        
        tunnelProcess.on('error', (err) => {
            clearTimeout(timeout);
            resolve({ success: false, error: err.message });
        });
    });
}

export async function deleteApp(agentId: number, userId: number, siteName: string): Promise<{ success: boolean }> {
    await stopAppServer(agentId, userId, siteName);
    await deleteAppServer(agentId, userId, siteName);
    
    const workspacePath = path.join(WORKSPACE_DIR, `${agentId}_${userId}`);
    const appPath = path.join(workspacePath, 'www', siteName);
    
    if (fs.existsSync(appPath)) {
        fs.rmSync(appPath, { recursive: true, force: true });
    }
    
    return { success: true };
}

export function getActiveAppCount(): number {
    return appProcesses.size;
}

export async function cleanupStaleServers(): Promise<void> {
    const activeServers = await getAllActiveAppServers();
    
    for (const server of activeServers) {
        const key = getProcessKey(server.agent_id, server.user_id, server.site_name);
        if (!appProcesses.has(key)) {
            await deactivateAppServer(server.agent_id, server.user_id, server.site_name);
        }
    }
}
