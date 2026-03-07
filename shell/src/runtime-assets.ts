import * as fs from 'fs';
import * as path from 'path';

function shouldCopy(sourcePath: string, targetPath: string): boolean {
    if (!fs.existsSync(targetPath)) return true;
    return fs.statSync(sourcePath).mtimeMs > fs.statSync(targetPath).mtimeMs;
}

export function syncRuntimeFile(sourcePath: string, targetPath: string): void {
    if (!fs.existsSync(sourcePath)) {
        throw new Error(`Missing runtime asset at ${sourcePath}`);
    }

    if (shouldCopy(sourcePath, targetPath)) {
        fs.copyFileSync(sourcePath, targetPath);
    }
}

export function ensureWorkspaceRuntimeAssets(workspacePath: string, runtimeDir: string = __dirname): void {
    const sourceAgentPath = path.join(runtimeDir, '../dist-agent/agent.js');
    const sourcePromptPath = path.join(runtimeDir, '../../system_prompt.txt');

    const targetAgentPath = path.join(workspacePath, 'agent.js');
    const targetPromptPath = path.join(workspacePath, 'system_prompt.txt');

    syncRuntimeFile(sourceAgentPath, targetAgentPath);
    syncRuntimeFile(sourcePromptPath, targetPromptPath);
}
