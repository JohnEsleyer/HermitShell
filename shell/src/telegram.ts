import { getAgentByToken, isAllowed, getBudget, updateSpend, canSpend, updateAuditLog, getAgentById, getSetting, getOperator, getActiveMeetings, createMeeting, updateMeetingTranscript, closeMeeting, getAllAgents, createAgentRuntimeLog } from './db';
import { spawnAgent, docker, getCubicleStatus, stopCubicle, removeCubicle, listContainers } from './docker';
import { claimDueCalendarEvents as wsClaimDueCalendarEvents, updateCalendarEvent as wsUpdateCalendarEvent, getCalendarEvents as wsGetCalendarEvents, createCalendarEvent as wsCreateCalendarEvent, deleteCalendarEvent as wsDeleteCalendarEvent, initWorkspaceDatabases, completeCalendarTaskHistory as wsCompleteCalendarTaskHistory } from './workspace-db';
import * as fs from 'fs';
import * as path from 'path';
import * as crypto from 'crypto';
import * as chokidar from 'chokidar';
import { loadHistory, saveHistory, clearHistory } from './history';
import { setPreviewPassword } from './server';
import { parseAgentResponse, parseFileAction, parseAppAction, normalizeAgentOutputToJson, extractCalendars } from './agent-response';
import { buildPublicAppEndpoint } from './sites';
import { startAppServer } from './app-server';
import { getTunnelUrl } from './tunnel';

interface TelegramUpdate {
    message?: {
        from: { id: number; username?: string; first_name?: string };
        text?: string;
        chat: { id: number };
        document?: { file_id: string; file_name?: string; mime_type?: string };
        photo?: { file_id: string }[];
        caption?: string;
    };
    callback_query?: {
        id: string;
        from: { id: number };
        data: string;
        message?: {
            chat: { id: number };
            message_id: number;
        };
    };
}

const TELEGRAM_MAX_LENGTH = 4000;

const WORKSPACE_DIR = path.join(__dirname, '../../data/workspaces');

const pendingDelegations = new Map<string, { agentId: number; role: string; task: string; timestamp: number }>();
const pendingInternetApprovals = new Map<number, { containerId: string; logId: number }>();
let calendarSchedulerStarted = false;



function sanitizeUserFacingOutput(output: string): string {
    const raw = String(output || '').trim();
    if (!raw) return raw;

    const hasLargeCode = raw.includes('```') || raw.includes('<!DOCTYPE html') || raw.includes('cat > ') || raw.includes('function ');
    if (raw.length > 1200 || hasLargeCode) {
        return '✅ Task completed. I kept implementation details in the workspace and will share files/URLs directly.';
    }

    return raw;
}

function buildAppUrl(baseUrl: string, agentId: number, userId: number, appName: string): string {
    return `${String(baseUrl).replace(/\/$/, '')}${buildPublicAppEndpoint(agentId, userId, appName)}`;
}

function getWorkspaceOutFiles(agentId: number, userId: number): string[] {
    const outPath = path.join(WORKSPACE_DIR, `${agentId}_${userId}`, 'out');
    if (!fs.existsSync(outPath)) return [];

    return fs.readdirSync(outPath)
        .filter((name) => !name.includes('..') && !name.includes('/') && !name.includes('\\'))
        .filter((name) => {
            const fullPath = path.join(outPath, name);
            return fs.existsSync(fullPath) && fs.statSync(fullPath).isFile();
        });
}

export async function sendChatAction(token: string, chatId: number, action: 'typing' | 'upload_document' = 'typing'): Promise<void> {
    const url = `https://api.telegram.org/bot${token}/sendChatAction`;

    try {
        await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                chat_id: chatId,
                action: action
            })
        });
    } catch (err) {
        console.error('Failed to send chat action:', err);
    }
}

export async function smartReply(token: string, chatId: number, text: string, messageId?: number): Promise<void> {
    if (text.length > TELEGRAM_MAX_LENGTH) {
        const buffer = Buffer.from(text, 'utf-8');
        const url = `https://api.telegram.org/bot${token}/sendDocument`;

        const formData = new FormData();
        formData.append('chat_id', String(chatId));
        formData.append('document', new Blob([buffer], { type: 'text/plain' }), 'output.txt');
        formData.append('caption', '📄 Output was too long, sent as file.');

        if (messageId) {
            try {
                await editMessageText(token, chatId, messageId, '✅ Response ready:');
            } catch { }
        }

        try {
            await fetch(url, {
                method: 'POST',
                body: formData
            });
        } catch (err) {
            console.error('Failed to send document:', err);
            const chunks = splitMessage(text);
            for (const chunk of chunks) {
                await sendTelegramMessage(token, chatId, chunk);
            }
        }
    } else {
        if (messageId) {
            const edited = await editMessageText(token, chatId, messageId, text);
            if (!edited) {
                await sendTelegramMessage(token, chatId, text);
            }
        } else {
            await sendTelegramMessage(token, chatId, text);
        }
    }
}

function splitMessage(text: string, maxLength: number = TELEGRAM_MAX_LENGTH): string[] {
    const chunks: string[] = [];
    let remaining = text;

    while (remaining.length > maxLength) {
        let splitPoint = maxLength;
        const newlineIndex = remaining.lastIndexOf('\n', maxLength);
        if (newlineIndex > maxLength * 0.5) {
            splitPoint = newlineIndex + 1;
        }

        chunks.push(remaining.slice(0, splitPoint));
        remaining = remaining.slice(splitPoint);
    }

    if (remaining.length > 0) {
        chunks.push(remaining);
    }

    return chunks;
}

export async function handleTelegramUpdate(token: string, update: TelegramUpdate): Promise<string | null> {
    if (update.callback_query) {
        return await handleCallbackQuery(token, update.callback_query);
    }

    const agent = await getAgentByToken(token);
    if (!agent) {
        console.log(`Unknown agent token: ${token.slice(0, 8)}...`);
        return null;
    }

    if (!update.message) {
        return null;
    }

    const userId = update.message.from.id;
    if (!await isAllowed(userId)) {
        await createAgentRuntimeLog(agent.id, 'warn', 'telegram', 'Unauthorized user blocked', { userId });
        const username = update.message.from.username || 'unknown';
        const firstName = update.message.from.first_name || '';
        console.log(`[UNAUTHORIZED] Telegram user tried to message bot:`);
        console.log(`  User ID: ${userId}`);
        console.log(`  Username: @${username}`);
        console.log(`  First Name: ${firstName}`);
        console.log(`  Add this user to allowlist in dashboard to grant access.`);

        return `🔒 *Access Required*

Your Telegram ID: \`${userId}\`

To get access, follow these steps:

*Step 1:* Open your web browser and go to the HermitShell Dashboard (ask your admin for the link)

*Step 2:* Log in with your admin credentials

*Step 3:* Click "Allowlist" in the left sidebar menu

*Step 4:* Click the "Add User" button

*Step 5:* Enter your Telegram ID: \`${userId}\`

*Step 6:* Click "Add User" to save

*Step 7:* Come back here and send /start again

---

💡 *Tip:* You can also ask the admin to add you. Just send them this message with your ID: \`${userId}\``;
    }

    if (!await canSpend(agent.id)) {
        return `❌ Budget exceeded for ${agent.name}. Please try again tomorrow.`;
    }

    const text = update.message.text;
    const chatId = update.message.chat.id;

    if (text) {
        const lowerText = text.trim().toLowerCase();
        if (pendingInternetApprovals.has(userId) && (lowerText === 'yes' || lowerText === 'no')) {
            const pending = pendingInternetApprovals.get(userId)!;
            try {
                const container = docker.getContainer(pending.containerId);
                const marker = lowerText === 'yes' ? '/tmp/hermit_approval.lock' : '/tmp/hermit_deny.lock';
                const exec = await container.exec({ Cmd: ['touch', marker], AttachStdout: true, AttachStderr: true });
                await exec.start({});

                if (lowerText === 'yes') {
                    await updateAuditLog(pending.logId, 'Network Allowed', userId);
                    await sendTelegramMessage(token, chatId, '✅ Access granted.');
                } else {
                    await updateAuditLog(pending.logId, 'Network Blocked', userId);
                    await sendTelegramMessage(token, chatId, '🚫 Access blocked.');
                }
            } catch (err) {
                await sendTelegramMessage(token, chatId, '❌ Failed to process approval decision.');
            } finally {
                pendingInternetApprovals.delete(userId);
            }
            return null;
        }
    }

    if (text === '/start') {
        const keyboard = {
            keyboard: [
                [{ text: '📊 Status' }, { text: '📁 Workspace' }],
                [{ text: '💰 Budget' }, { text: '🔄 Reset' }],
                [{ text: '❓ Help' }]
            ],
            resize_keyboard: true,
            one_time_keyboard: false
        };

        const url = `https://api.telegram.org/bot${token}/sendMessage`;
        await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                chat_id: chatId,
                text: `🦀 *Welcome to HermitShell!*\n\nI'm *${agent.name}*, your AI assistant.\n\n*Role:* ${agent.role || 'General'}\n\nUse the menu below or just send me a message to start!`,
                parse_mode: 'Markdown',
                reply_markup: keyboard
            })
        });
        return null;
    }

    if (text === '📊 Status') {
        return await handleStatusCommand(agent, userId);
    }
    if (text === '📁 Workspace') {
        return await handleWorkspaceCommand(agent, userId);
    }
    if (text === '💰 Budget') {
        return await handleBudgetCommand(agent);
    }
    if (text === '🔄 Reset') {
        return await handleResetCommand(agent, userId);
    }
    if (text === '❓ Help') {
        return await handleHelpCommand(agent);
    }

    if (text === '/help') {
        return await handleHelpCommand(agent);
    }

    if (update.message.document || update.message.photo) {
        return await handleFileUpload(token, agent, userId, update.message);
    }

    if (text === '/status') {
        return await handleStatusCommand(agent, userId);
    }

    if (text === '/debug') {
        const status = await getCubicleStatus(agent.id, userId);
        const budget = await getBudget(agent.id);
        const settings = await import('./db').then(m => m.getAllSettings());

        let workspaceFiles = 'N/A';
        const workspacePath = path.join(__dirname, '../../data/workspaces', `${agent.id}_${userId}`);
        if (fs.existsSync(workspacePath)) {
            try {
                const files = fs.readdirSync(workspacePath);
                workspaceFiles = files.length > 0 ? files.slice(0, 10).join(', ') : '(empty)';
                if (files.length > 10) workspaceFiles += ` ... +${files.length - 10} more`;
            } catch (e) {
                workspaceFiles = 'Error reading';
            }
        }

        let terminalLink = '';
        if (status?.containerId && settings.public_url) {
            terminalLink = `\n\n🌐 *Terminal:* ${settings.public_url}/dashboard/ → Cubicles → Terminal`;
        }

        return `🔍 *Debug Info for ${agent.name}*

*Agent:*
• ID: ${agent.id}
• Name: ${agent.name}
• Role: ${agent.role || 'None'}
• Network HITL: Internet-only approval

*Cubicle:*
• Status: ${status?.status || 'None'}
• Container: \`${status?.containerId?.slice(0, 12) || 'N/A'}\`

*Budget:*
• Limit: $${budget?.daily_limit_usd || 1}/day
• Spent: $${budget?.current_spend_usd?.toFixed(4) || 0}

*Workspace:*
• Path: ${workspacePath}
• Files: ${workspaceFiles}

*User:*
• Telegram ID: ${userId}

*System:*
• Public URL: ${settings.public_url || 'Not set'}
• Default Model: ${settings.default_model || 'openrouter/free'}${terminalLink}`;
    }

    if (text === '/logs') {
        const workspacePath = path.join(__dirname, '../../data/workspaces', `${agent.id}_${userId}`);
        const logFilePath = path.join(workspacePath, '.hermit.log');

        if (!fs.existsSync(logFilePath)) {
            return `📝 No logs found yet. Send a message to the agent first!`;
        }

        try {
            const logs = fs.readFileSync(logFilePath, 'utf-8');
            const lines = logs.split('\n');
            const recentLogs = lines.slice(-30).join('\n');
            const cleanLogs = recentLogs.replace(/\x1b\[[0-9;]*m/g, '');

            return `📋 *Internal Agent Logs (Last 30 lines):*\n\`\`\`\n${cleanLogs || 'Empty log file'}\n\`\`\``;
        } catch (e: any) {
            return `❌ Failed to read logs: ${e.message}`;
        }
    }

    if (text === '/workspace') {
        return await handleWorkspaceCommand(agent, userId);
    }

    if (text === '/reset') {
        return await handleResetCommand(agent, userId);
    }

    if (text === '/clear') {
        clearHistory(`telegram_${agent.id}_${userId}`);
        return '🧹 Conversation context cleared. I will respond without previous chat memory.';
    }

    if (text === '/tokens') {
        const historyKey = `telegram_${agent.id}_${userId}`;
        const history = loadHistory(historyKey);
        const systemPromptLength = (agent.system_prompt || '').length;
        
        let totalChars = systemPromptLength;
        history.forEach(m => totalChars += String(m.content || '').length);
        
        const estimatedTokens = Math.ceil(totalChars / 4);
        
        return `📊 *Context Window Info*\n\n` +
               `*Estimated Tokens:* ~${estimatedTokens}\n` +
               `*Messages in memory:* ${history.length}\n\n` +
               `_Tip: Use /clear to reset the context window and save tokens._`;
    }

    if (text === '/budget') {
        return await handleBudgetCommand(agent);
    }

    if (text?.startsWith('/containers') || text === '/containers') {
        const isOperator = (await getOperator())?.user_id === userId;
        if (!isOperator) {
            return `❌ Operator only command.`;
        }

        const containers = await listContainers();
        if (containers.length === 0) {
            return `📦 No containers running.`;
        }

        const lines = containers.slice(0, 10).map(c => {
            const status = c.State === 'running' ? '🟢' : c.State === 'exited' ? '🔴' : '🟡';
            const name = c.Names?.[0]?.replace('/', '') || c.Id.slice(0, 12);
            return `${status} ${name} (${c.Image})`;
        });

        return `📦 *All Containers (${containers.length})*\n\n${lines.join('\n')}` +
            (containers.length > 10 ? `\n... +${containers.length - 10} more` : '');
    }

    if (text?.startsWith('/agents') || text === '/agents') {
        const isOperator = (await getOperator())?.user_id === userId;
        if (!isOperator) {
            return `❌ Operator only command.`;
        }

        const agents = await getAllAgents();
        const lines = agents.map(a => {
            const status = a.is_active ? '🟢' : '🔴';
            const hitl = a.require_approval ? '🔒' : '';
            return `${status} ${a.name} - ${a.role || 'No role'} ${hitl}`;
        });

        return `🤖 *All Agents (${agents.length})*\n\n${lines.join('\n')}`;
    }

    console.log(`[${agent.name}] Processing: ${text}`);

    return null;
}

export async function processAgentMessage(
    token: string,
    chatId: number,
    userId: number,
    text: string,
    statusMessageId?: number
): Promise<{ output: string; messageId?: number }> {
    const agent = await getAgentByToken(token);
    if (!agent) {
        return { output: 'Agent not found.' };
    }

    await createAgentRuntimeLog(agent.id, 'info', 'telegram', 'Message received', { userId, chatId, preview: text.slice(0, 120) });

    await sendChatAction(token, chatId, 'typing');

    if (statusMessageId) {
        await editMessageText(token, chatId, statusMessageId, `🔄 *${agent.name}* is waking up...`);
    }

    const historyKey = `telegram_${agent.id}_${userId}`;
    const history = loadHistory(historyKey);

    const meetings = await getActiveMeetings(agent.id);
    const meetingContext = meetings.length > 0
        ? meetings.map(m => `Meeting with Agent ${m.initiator_id === agent.id ? m.participant_id : m.initiator_id}: ${m.topic}\n${m.transcript || 'No transcript yet'}`).join('\n\n')
        : null;
    const beforeOutFiles = new Set(getWorkspaceOutFiles(agent.id, userId));

    try {
        if (statusMessageId) {
            await editMessageText(token, chatId, statusMessageId, `🔄 *${agent.name}* is thinking...`);
        }

        const result = await spawnAgent({
            agentId: agent.id,
            agentName: agent.name,
            agentRole: agent.role,
            dockerImage: agent.docker_image,
            userMessage: text,
            history: history.slice(-10),
            maxTokens: 1000,
            requireApproval: agent.require_approval === 1,
            userId: userId,
            llmProvider: agent.llm_provider && agent.llm_provider !== 'default' ? agent.llm_provider : undefined,
            llmModel: agent.llm_model && agent.llm_model !== 'default' ? agent.llm_model : undefined,
            personality: agent.personality,
            onProgress: async (status: string, details?: string) => {
                if (statusMessageId) {
                    const settings = await import('./db').then(m => m.getAllSettings());
                    let msg = details
                        ? `${status}\n\`${details}\``
                        : status;

                    await editMessageText(token, chatId, statusMessageId, msg);
                }
                await sendChatAction(token, chatId, 'typing');
            }
        });

        if (result.output.includes('401') && (result.output.includes('Unauthorized') || result.output.includes('Authentication'))) {
            result.output = `❌ *API Key Error (401 Unauthorized)*\n\nYour API key is either missing or invalid for this provider.\n\n*How to fix:*\n1. Open Dashboard -> Settings\n2. Enter a valid API key\n3. Click "Save All Settings"\n4. Send \`/reset\` here to delete this broken cubicle and apply your new keys!`;
        }

        history.push({ role: 'user', content: text });
        history.push({ role: 'assistant', content: normalizeAgentOutputToJson(result.output, userId) });
        saveHistory(historyKey, history.slice(-40));

        const previewInfo = detectWebServer(result.output, agent.id);
        if (previewInfo) {
            await sendPreviewButton(token, chatId, previewInfo.url, previewInfo.port, agent.id);
        }

        if (result.output.includes('[MEETING]') && result.output.includes('TARGET_ROLE:')) {
            const roleMatch = result.output.match(/TARGET_ROLE:\s*(.+)/);
            const taskMatch = result.output.match(/TASK:\s*(.+)/);

            if (roleMatch && taskMatch) {
                const targetRole = roleMatch[1].trim();
                const task = taskMatch[1].trim();

                const delegationId = `${agent.id}_${Date.now()}`;
                pendingDelegations.set(delegationId, {
                    agentId: agent.id,
                    role: targetRole,
                    task: task,
                    timestamp: Date.now()
                });

                await sendDelegationRequest(token, agent.id, agent.name, targetRole, task, delegationId);

                return {
                    output: `📋 Delegation request sent to operator for approval.\nTarget Role: *${targetRole}*\nTask: ${task.substring(0, 100)}...`,
                    messageId: statusMessageId
                };
            }
        }

        let finalOutput = result.output;
        const parsed = parseAgentResponse(result.output);
        const calendars = extractCalendars(result.output);

        if (calendars.length > 0) {
            for (const cal of calendars) {
                await initWorkspaceDatabases(agent.id, userId);
                await wsCreateCalendarEvent({
                    agent_id: agent.id,
                    title: 'Scheduled Task',
                    prompt: cal.prompt,
                    start_time: cal.datetime,
                    target_user_id: userId,
                    symbol: '⏰'
                }, userId);
            }
            finalOutput = finalOutput.replace(/<calendar>[\s\S]*?<\/calendar>/gi, '').trim();
            finalOutput += `\n\n⏰ Added ${calendars.length} scheduled event(s) to the calendar.`;
        }

        if (parsed.message) {
            finalOutput = parsed.message + (calendars.length > 0 ? `\n\n⏰ Added ${calendars.length} scheduled event(s) to the calendar.` : '');
        }
        finalOutput = sanitizeUserFacingOutput(finalOutput);

        const selectedFile = parseFileAction(parsed.action);
        let deliveredByAction = false;
        if (selectedFile) {
            const outPath = path.join(WORKSPACE_DIR, `${agent.id}_${userId}`, 'out');
            const filePath = path.join(outPath, selectedFile);
            if (fs.existsSync(filePath) && fs.statSync(filePath).isFile() && !processedFiles.has(filePath)) {
                processedFiles.add(filePath);
                await sendFileViaTelegram(token, chatId, filePath, `📎 ${selectedFile}`);
                deliveredByAction = true;
                setTimeout(() => processedFiles.delete(filePath), 30000);
            }
        }

        const afterOutFiles = getWorkspaceOutFiles(agent.id, userId);
        const autoDiscoveredFiles = afterOutFiles.filter((name) => !beforeOutFiles.has(name));
        if (!selectedFile && autoDiscoveredFiles.length > 0) {
            const newestFirst = autoDiscoveredFiles
                .map((name) => ({
                    name,
                    fullPath: path.join(WORKSPACE_DIR, `${agent.id}_${userId}`, 'out', name),
                    mtime: fs.statSync(path.join(WORKSPACE_DIR, `${agent.id}_${userId}`, 'out', name)).mtimeMs
                }))
                .sort((a, b) => b.mtime - a.mtime)
                .slice(0, 3);

            for (const file of newestFirst) {
                if (processedFiles.has(file.fullPath)) continue;
                processedFiles.add(file.fullPath);
                await sendFileViaTelegram(token, chatId, file.fullPath, `📎 ${file.name}`);
                setTimeout(() => processedFiles.delete(file.fullPath), 30000);
            }

            const names = newestFirst.map((f) => f.name).join(', ');
            finalOutput += `\n\n📎 Auto-delivered file(s): ${names}`;
        } else if (deliveredByAction && selectedFile) {
            finalOutput += `\n\n📎 Delivered file: ${selectedFile}`;
        }

        const selectedApp = parseAppAction(parsed.action);
        if (selectedApp) {
            const started = await startAppServer(agent.id, userId, selectedApp);
            if (!started.success) {
                finalOutput += `

❌ APP publish failed for ${selectedApp}: ${started.error || 'unknown error'}`;
            } else {
                const settings = await import('./db').then(m => m.getAllSettings());
                const baseUrl = settings.public_url || getTunnelUrl() || 'http://localhost:3000';
                const appUrl = buildAppUrl(baseUrl, agent.id, userId, selectedApp);
                finalOutput += `

🌐 App is ready: ${appUrl}`;
            }
        }

        try {
            const workspaceWwwPath = path.join(WORKSPACE_DIR, `${agent.id}_${userId}`, 'www');
            if (fs.existsSync(workspaceWwwPath) && fs.statSync(workspaceWwwPath).isDirectory()) {
                const settings = await import('./db').then(m => m.getAllSettings());
                const baseUrl = settings.public_url || getTunnelUrl() || 'http://localhost:3000';
                const appFolders = fs.readdirSync(workspaceWwwPath, { withFileTypes: true })
                    .filter((entry) => entry.isDirectory())
                    .map((entry) => entry.name)
                    .filter((name) => fs.existsSync(path.join(workspaceWwwPath, name, 'index.html')));

                if (appFolders.length > 0) {
                    const urls = appFolders.map((name) => `• ${name}: ${buildAppUrl(baseUrl, agent.id, userId, name)}`);
                    finalOutput += `

🌐 Available app URLs:
${urls.join('\n')}`;
                }
            }
        } catch {}

        const estimatedCost = result.output.length * 0.00001;
        await updateSpend(agent.id, estimatedCost);

        return { output: finalOutput, messageId: statusMessageId };
    } catch (error: any) {
        console.error(`[${agent.name}] Error:`, error);
        await createAgentRuntimeLog(agent.id, 'error', 'telegram', error.message || 'Agent processing failed', { userId, chatId });
        return { output: `Error: ${error.message}`, messageId: statusMessageId };
    }
}

async function sendDelegationRequest(
    token: string,
    agentId: number,
    agentName: string,
    targetRole: string,
    task: string,
    delegationId: string
): Promise<void> {
    const operator = await getOperator();
    if (!operator) {
        console.log('No operator configured for delegation request');
        return;
    }

    const keyboard = {
        inline_keyboard: [[
            { text: '✅ Approve Delegation', callback_data: `delegate_approve:${delegationId}:${agentId}` },
            { text: '❌ Deny', callback_data: `delegate_deny:${delegationId}:${agentId}` }
        ]]
    };

    const url = `https://api.telegram.org/bot${token}/sendMessage`;

    await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            chat_id: operator.user_id,
            text: `🤝 *Delegation Request*\n\nAgent *${agentName}* wants to delegate a sub-task:\n\n*Target Role:* ${targetRole}\n*Task:*\n\`\`\`\n${task.substring(0, 500)}\n\`\`\`\n\nThis will spawn a new cubicle for the sub-task.`,
            parse_mode: 'Markdown',
            reply_markup: keyboard
        })
    });
}

async function handleCallbackQuery(token: string, query: TelegramUpdate['callback_query']): Promise<string | null> {
    if (!query?.data || !query.message) return null;

    const parts = query.data.split(':');
    const action = parts[0];
    const chatId = query.message.chat.id;
    const messageId = query.message.message_id;

    if (action === 'delegate_approve' || action === 'delegate_deny') {
        const delegationId = parts[1];
        const agentId = parseInt(parts[2], 10);
        const adminId = query.from.id;

        const delegation = pendingDelegations.get(delegationId);

        if (!delegation) {
            await editMessageText(token, chatId, messageId, `❌ Delegation request expired or not found.`);
            return null;
        }

        if (action === 'delegate_approve') {
            await editMessageText(token, chatId, messageId, `✅ *Delegation Approved!*\n\nSpawning sub-agent for: ${delegation.role}...`);

            const agent = await getAgentById(agentId);
            if (agent) {
                try {
                    const result = await spawnAgent({
                        agentId: agentId,
                        agentName: delegation.role,
                        agentRole: delegation.role,
                        dockerImage: agent.docker_image,
                        userMessage: delegation.task,
                        history: [],
                        maxTokens: 1000,
                        requireApproval: false,
                        userId: adminId
                    });

                    await sendTelegramMessage(token, chatId, `🤝 Sub-agent completed:\n\n${result.output.substring(0, 3000)}`);
                } catch (err: any) {
                    await sendTelegramMessage(token, chatId, `❌ Delegation failed: ${err.message}`);
                }
            }
        } else {
            await editMessageText(token, chatId, messageId, `❌ *Delegation Denied.* No sub-agent will be spawned.`);
        }

        pendingDelegations.delete(delegationId);
        return null;
    }

    const logIdStr = parts[1];
    const containerId = parts[2];
    const logId = parseInt(logIdStr, 10);

    if (action === 'approve' || action === 'deny') {
        const status = action === 'approve' ? 'Network Allowed' : 'Network Blocked';

        if (!isNaN(logId)) {
            await updateAuditLog(logId, status, query.from.id);
        }

        if (action === 'approve' && containerId) {
            try {
                const container = docker.getContainer(containerId);
                const exec = await container.exec({
                    Cmd: ['touch', '/tmp/hermit_approval.lock'],
                    AttachStdout: true,
                    AttachStderr: true
                });
                await exec.start({});
                await editMessageText(token, chatId, messageId, `✅ *Approved!* Network command execution started.`);
            } catch (err) {
                console.error('Failed to approve command:', err);
                await editMessageText(token, chatId, messageId, `❌ Approval applied but container may not have received it.`);
            }
        } else {
            await editMessageText(token, chatId, messageId, `❌ *Denied.* Command will not be executed.`);

            if (containerId) {
                try {
                    const container = docker.getContainer(containerId);
                    const exec = await container.exec({
                        Cmd: ['touch', '/tmp/hermit_deny.lock'],
                        AttachStdout: true,
                        AttachStderr: true
                    });
                    await exec.start({});
                } catch { }
            }
        }

        return null;
    }

    return null;
}

export async function sendVerificationCode(token: string, chatId: number, code: string): Promise<boolean> {
    const url = `https://api.telegram.org/bot${token}/sendMessage`;

    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                chat_id: chatId,
                text: `🛡️ *HermitShell Agent Verification*\n\nYou are linking this bot to the Orchestrator.\n\nYour verification code is: \`${code}\`\n\nEnter this code in the dashboard to complete setup.`,
                parse_mode: 'Markdown'
            })
        });

        return response.ok;
    } catch (err) {
        console.error('Failed to send verification code:', err);
        return false;
    }
}

export async function sendApprovalRequest(
    agentId: number,
    containerId: string,
    command: string,
    logId: number
): Promise<void> {
    const agent = await getAgentById(agentId);
    if (!agent) return;

    const operator = await getOperator();
    const adminChatId = operator?.user_id || await getSetting('admin_chat_id');

    if (!adminChatId) {
        console.log('No operator/admin chat ID configured');
        return;
    }

    const tgToken = agent.telegram_token;
    if (!tgToken) return;

    pendingInternetApprovals.set(Number(adminChatId), { containerId, logId });

    const url = `https://api.telegram.org/bot${tgToken}/sendMessage`;

    await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            chat_id: adminChatId,
            text: `⚠️ *Network Access Requested*

The agent wants to access the internet to run:
\`${command}\`

Reply with *Yes* to allow, or *No* to block.`,
            parse_mode: 'Markdown'
        })
    });
}

export async function sendTelegramMessage(token: string, chatId: number, text: string): Promise<number | undefined> {
    const url = `https://api.telegram.org/bot${token}/sendMessage`;

    const send = async (payload: Record<string, any>) => {
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        return await response.json() as { ok: boolean; description?: string; result?: { message_id: number } };
    };

    try {
        let data = await send({ chat_id: chatId, text, parse_mode: 'Markdown' });
        if (!data.ok) {
            data = await send({ chat_id: chatId, text });
        }
        return data.result?.message_id;
    } catch (err) {
        console.error('Failed to send Telegram message:', err);
        return undefined;
    }
}

export function startCalendarScheduler(): void {
    if (calendarSchedulerStarted) return;
    calendarSchedulerStarted = true;

    const tick = async () => {
        try {
            const agents = await getAllAgents();
            const workspaceDir = path.join(__dirname, '../../data/workspaces');
            
            if (!fs.existsSync(workspaceDir)) return;

            const entries = fs.readdirSync(workspaceDir, { withFileTypes: true });
            for (const entry of entries) {
                if (!entry.isDirectory()) continue;
                
                const parts = entry.name.split('_');
                if (parts.length !== 2) continue;
                
                const agentId = parseInt(parts[0]);
                const userId = parseInt(parts[1]);
                if (isNaN(agentId) || isNaN(userId)) continue;

                const agent = agents.find(a => a.id === agentId);
                if (!agent) continue;

                let dueEvents;
                try {
                    dueEvents = await wsClaimDueCalendarEvents(agentId, userId);
                } catch (err: any) {
                    if (err?.code === 'SQLITE_ERROR' && err?.message?.includes('no such table')) {
                        continue;
                    }
                    throw err;
                }

                for (const event of dueEvents) {
                    try {
                        if (!agent.telegram_token) {
                            await wsUpdateCalendarEvent(event.id, agentId, {
                                status: 'failed',
                                completed_at: new Date().toISOString(),
                                last_error: 'Agent missing Telegram token'
                            }, userId);
                            continue;
                        }

                        const eventMarker = event.symbol || '📅';
                        const isInternal = event.prompt.startsWith('INTERNAL:') || event.title.includes('[INTERNAL]');

                        if (!isInternal) {
                            await sendTelegramMessage(agent.telegram_token, event.target_user_id, `${eventMarker} *Scheduled Event:* ${event.title}`);
                        }

                        const result = await processAgentMessage(agent.telegram_token, event.target_user_id, event.target_user_id, event.prompt);

                        if (result.output) {
                            await smartReply(agent.telegram_token, event.target_user_id, result.output);
                        }

                        await wsCompleteCalendarTaskHistory(event as any, agentId, userId, true, result.output || 'ok');
                    } catch (e: any) {
                        await wsCompleteCalendarTaskHistory(event as any, agentId, userId, false, String(e?.message || e));
                    }
                }
            }
        } catch (err) {
            console.error('[CalendarScheduler] Tick error:', err);
        }
    };

    tick().catch((e) => console.error('[CalendarScheduler] Initial tick error:', e));
    setInterval(() => tick().catch((e) => console.error('[CalendarScheduler] Tick error:', e)), 30000);
}

export async function editMessageText(token: string, chatId: number, messageId: number, text: string): Promise<boolean> {
    const url = `https://api.telegram.org/bot${token}/editMessageText`;

    const edit = async (payload: Record<string, any>) => {
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        return await response.json() as { ok: boolean; description?: string };
    };

    try {
        let data = await edit({
            chat_id: chatId,
            message_id: messageId,
            text: text,
            parse_mode: 'Markdown'
        });

        if (!data.ok) {
            if (data.description?.includes('message is not modified')) {
                return true;
            }
            data = await edit({
                chat_id: chatId,
                message_id: messageId,
                text: text
            });
        }

        if (!data.ok) {
            if (data.description?.includes('message is not modified')) {
                return true;
            }
            console.error('Failed to edit Telegram message:', data.description || 'Unknown API error');
            return false;
        }

        return true;
    } catch (err) {
        console.error('Failed to edit message:', err);
        return false;
    }
}

export async function sendFileViaTelegram(token: string, chatId: number, filePath: string, caption?: string): Promise<boolean> {
    if (!fs.existsSync(filePath)) {
        console.error(`[File] File not found: ${filePath}`);
        return false;
    }

    const stat = fs.statSync(filePath);
    if (stat.size > 50 * 1024 * 1024) {
        console.error(`[File] File too large: ${filePath} (${stat.size} bytes)`);
        return false;
    }

    const url = `https://api.telegram.org/bot${token}/sendDocument`;
    const buffer = fs.readFileSync(filePath);
    const filename = path.basename(filePath);

    const formData = new FormData();
    formData.append('chat_id', String(chatId));
    formData.append('document', new Blob([buffer]), filename);
    if (caption) formData.append('caption', caption);

    try {
        await sendChatAction(token, chatId, 'upload_document');
        const response = await fetch(url, { method: 'POST', body: formData });
        const data = await response.json() as any;
        if (!data.ok) {
            console.error(`[File] Failed to send: ${data.description}`);
            return false;
        }
        console.log(`[File] Sent: ${filename}`);
        return true;
    } catch (err) {
        console.error(`[File] Error sending: ${err}`);
        return false;
    }
}

let fileWatcher: chokidar.FSWatcher | null = null;
const processedFiles = new Set<string>();

export function startFileWatcher() {
    if (fileWatcher) return;

    console.log('[FileWatcher] Initializing autonomous file portal monitor...');

    // Pre-populate processedFiles with existing files so we don't dump everything on restart
    try {
        const workspaceDirs = fs.readdirSync(WORKSPACE_DIR);
        for (const dir of workspaceDirs) {
            const outPath = path.join(WORKSPACE_DIR, dir, 'out');
            if (fs.existsSync(outPath)) {
                const files = fs.readdirSync(outPath);
                for (const file of files) {
                    processedFiles.add(path.join(outPath, file));
                }
            }
        }
    } catch (e) {
        console.error('[FileWatcher] Error pre-populating files:', e);
    }

    fileWatcher = chokidar.watch(path.join(WORKSPACE_DIR, '**/out/**/*'), {
        persistent: true,
        ignoreInitial: true,
        awaitWriteFinish: { stabilityThreshold: 2000, pollInterval: 500 }
    });

    fileWatcher.on('add', async (filePath) => {
        if (processedFiles.has(filePath)) return;
        processedFiles.add(filePath);

        const parts = filePath.split(path.sep);
        const outIndex = parts.indexOf('out');
        if (outIndex > 0) {
            const workspaceId = parts[outIndex - 1];
            const [agentIdStr, userIdStr] = workspaceId.split('_');
            const agentId = Number(agentIdStr);
            const chatId = Number(userIdStr);

            try {
                const agent = await getAgentById(agentId);
                if (agent && agent.telegram_token) {
                    await sendFileViaTelegram(agent.telegram_token, chatId, filePath, `📎 ${path.basename(filePath)} (Detected)`);
                }
            } catch (e) {
                console.error('[FileWatcher] Error sending file:', e);
            }
        }

        // Clear from Set after a few seconds in case it gets rewritten
        setTimeout(() => processedFiles.delete(filePath), 10000);
    });
}

export async function registerWebhook(token: string, baseUrl: string, secret: string): Promise<boolean> {
    const cleanBaseUrl = baseUrl.replace(/\/$/, '');
    const cleanSecret = secret.replace(/[^a-zA-Z0-9_-]/g, '') || 'hermitSecret123';
    const maxDnsRetryAttempts = 6;

    try {
        await fetch(`https://api.telegram.org/bot${token}/deleteWebhook`);
    } catch { }

    for (let attempt = 1; attempt <= maxDnsRetryAttempts; attempt++) {
        try {
            const webhookUrl = `${cleanBaseUrl}/webhook/${token}?secret=${encodeURIComponent(cleanSecret)}`;
            const tgUrl = `https://api.telegram.org/bot${token}/setWebhook?url=${encodeURIComponent(webhookUrl)}&secret_token=${cleanSecret}`;

            const response = await fetch(tgUrl);
            const data = await response.json() as any;

            if (data.ok) {
                await setBotCommands(token);
                return true;
            }

            const description = String(data?.description || '').toLowerCase();
            const isDnsPropagationError = description.includes('failed to resolve host');

            if (isDnsPropagationError && attempt < maxDnsRetryAttempts) {
                const delayMs = 4000 * attempt;
                console.warn(`[Webhook] Hostname propagation pending for ${token.substring(0, 8)}... retrying in ${delayMs}ms (attempt ${attempt}/${maxDnsRetryAttempts})`);
                await new Promise((resolve) => setTimeout(resolve, delayMs));
                continue;
            }

            console.error(`Failed to set webhook for token ${token.substring(0, 8)}...:`, data);
            return false;
        } catch (e) {
            console.error('Error setting webhook:', e);
            return false;
        }
    }

    return false;
}

export async function setBotCommands(token: string): Promise<void> {
    const url = `https://api.telegram.org/bot${token}/setMyCommands`;

    try {
        await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                commands: [
                    { command: 'start', description: 'Welcome message & help' },
                    { command: 'help', description: 'Show all commands' },
                    { command: 'status', description: 'Check cubicle status' },
                    { command: 'debug', description: 'Detailed debug info' },
                    { command: 'logs', description: 'View container logs' },
                    { command: 'workspace', description: 'List workspace files' },
                    { command: 'budget', description: 'Check remaining budget' },
                    { command: 'tokens', description: 'Check estimated token usage' },
                    { command: 'reset', description: 'Reset the cubicle' },
                    { command: 'clear', description: 'Clear conversation context' },
                    { command: 'containers', description: 'List all containers (operator)' },
                    { command: 'agents', description: 'List all agents (operator)' }
                ]
            })
        });
    } catch (err) {
        console.error('Failed to set bot commands:', err);
    }
}

async function handleStatusCommand(agent: any, userId: number): Promise<string> {
    const status = await getCubicleStatus(agent.id, userId);
    if (!status) {
        return `📊 *Cubicle Status: None*\n\nNo container exists yet.\nSend me a message to spawn one!`;
    }
    const statusEmoji = status.status === 'running' ? '🟢' : status.status === 'exited' ? '🔴' : '🟡';
    return `${statusEmoji} *Cubicle Status: ${status.status.toUpperCase()}*\n\n` +
        `*Agent:* ${agent.name}\n` +
        `*Container:* \`${status.containerId?.slice(0, 12) || 'N/A'}\`\n` +
        `*Image:* ${agent.docker_image}`;
}

async function handleWorkspaceCommand(agent: any, userId: number): Promise<string> {
    const workspacePath = path.join(__dirname, '../../data/workspaces', `${agent.id}_${userId}`);

    if (!fs.existsSync(workspacePath)) {
        return `📁 Workspace not created yet.\n\nSend a message to spawn a container and create the workspace.`;
    }

    try {
        const listFiles = (dir: string, prefix: string = ''): string[] => {
            const items: string[] = [];
            const files = fs.readdirSync(dir);
            for (const file of files) {
                const fullPath = path.join(dir, file);
                const stat = fs.statSync(fullPath);
                if (stat.isDirectory()) {
                    items.push(`${prefix}📁 ${file}/`);
                    const subFiles = listFiles(fullPath, prefix + '  ');
                    if (subFiles.length > 0) items.push(...subFiles.slice(0, 5));
                } else {
                    const size = stat.size < 1024 ? `${stat.size}B` : `${(stat.size / 1024).toFixed(1)}KB`;
                    items.push(`${prefix}📄 ${file} (${size})`);
                }
            }
            return items;
        };

        const files = listFiles(workspacePath);
        const fileList = files.slice(0, 20).join('\n') || '(empty)';
        const more = files.length > 20 ? `\n... +${files.length - 20} more` : '';

        return `📁 *Workspace*\n\`${workspacePath}\`\n\n${fileList}${more}`;
    } catch (e: any) {
        return `❌ Error reading workspace: ${e.message}`;
    }
}

async function handleResetCommand(agent: any, userId: number): Promise<string> {
    const workspacePath = path.join(WORKSPACE_DIR, `${agent.id}_${userId}`);
    const status = await getCubicleStatus(agent.id, userId);

    if (status?.containerId) {
        try {
            await removeCubicle(status.containerId);
        } catch (e: any) {
            return `❌ Failed to reset container: ${e.message}`;
        }
    }

    if (fs.existsSync(workspacePath)) {
        try {
            fs.rmSync(workspacePath, { recursive: true, force: true });
        } catch (e: any) {
            return `❌ Failed to wipe workspace data: ${e.message}`;
        }
    }

    return `🔄 *Container Fully Reset*\n\nCubicle removed and workspace deleted.\nSend a message to spawn a fresh one.`;
}

async function handleBudgetCommand(agent: any): Promise<string> {
    const budget = await getBudget(agent.id);
    if (budget) {
        const percent = (budget.current_spend_usd / budget.daily_limit_usd * 100).toFixed(1);
        const bar = '█'.repeat(Math.min(10, Math.floor(parseFloat(percent) / 10))) + '░'.repeat(10 - Math.min(10, Math.floor(parseFloat(percent) / 10)));
        return `💰 *Budget for ${agent.name}*\n\n` +
            `*Limit:* $${budget.daily_limit_usd.toFixed(2)}/day\n` +
            `*Spent:* $${budget.current_spend_usd.toFixed(4)}\n` +
            `*Remaining:* $${(budget.daily_limit_usd - budget.current_spend_usd).toFixed(4)}\n\n` +
            `[${bar}] ${percent}%`;
    }
    return `Budget info not available.`;
}

async function handleHelpCommand(agent: any): Promise<string> {
    return `🦀 *HermitShell Commands*

*Agent Commands:*
/status - Cubicle status (running/stopped)
/debug - Full debug info (container, workspace, etc.)
/logs - Recent container logs
/workspace - Files in persistent workspace
/budget - Daily budget remaining
/tokens - Check estimated token usage
/reset - Kill and reset cubicle
/clear - Clear conversation context

*How it works:*
1. Send any message → I wake up or spawn a container
2. Container runs continuously and processes your requests
3. Files in /workspace persist across sessions
4. Use Dashboard to manually Start/Stop/Delete containers

*Your Agent:* ${agent.name}
*Role:* ${agent.role || 'General'}
*Image:* ${agent.docker_image}`;
}

async function handleFileUpload(token: string, agent: any, userId: number, message: any): Promise<string> {
    const workspacePath = path.join(__dirname, '../../data/workspaces', `${agent.id}_${userId}`);
    if (!fs.existsSync(workspacePath)) {
        fs.mkdirSync(workspacePath, { recursive: true });
    }

    try {
        let fileId: string;
        let fileName: string;

        if (message.document) {
            fileId = message.document.file_id;
            fileName = message.document.file_name || 'uploaded_file';
        } else if (message.photo && message.photo.length > 0) {
            fileId = message.photo[message.photo.length - 1].file_id;
            fileName = `photo_${Date.now()}.jpg`;
        } else {
            return `❌ Could not process file.`;
        }

        const fileUrl = `https://api.telegram.org/bot${token}/getFile?file_id=${fileId}`;
        const fileResponse = await fetch(fileUrl);
        const fileData = await fileResponse.json() as any;

        if (!fileData.ok) {
            return `❌ Failed to get file info.`;
        }

        const filePath = fileData.result.file_path;
        const downloadUrl = `https://api.telegram.org/file/bot${token}/${filePath}`;
        const downloadResponse = await fetch(downloadUrl);
        const buffer = Buffer.from(await downloadResponse.arrayBuffer());

        const inboundDir = path.join(workspacePath, 'in');
        fs.mkdirSync(inboundDir, { recursive: true });
        const savePath = path.join(inboundDir, fileName);
        fs.writeFileSync(savePath, buffer);

        return `✅ *File uploaded successfully!*\n\n📄 \`${fileName}\`\nSaved to workspace. I can now access it.`;
    } catch (e: any) {
        return `❌ Failed to upload file: ${e.message}`;
    }
}

export function escapeMarkdown(text: string): string {
    return text.replace(/([_*\[\]()~`>#+\-=|{}.!\\])/g, '\\$1');
}

export function detectWebServer(output: string, agentId: number): { url: string; port: number } | null {
    const lines = output.split(/\r?\n/);

    for (const line of lines) {
        const lower = line.toLowerCase();
        const hasServerSignal =
            lower.includes('http://') ||
            lower.includes('https://') ||
            lower.includes('listening') ||
            lower.includes('serving') ||
            lower.includes('started server') ||
            lower.includes('running on');

        if (!hasServerSignal) continue;

        const match = line.match(/(?:http:\/\/)?(?:0\.0\.0\.0|localhost|127\.0\.0\.1):(\d{2,5})/i)
            || line.match(/(?:port|: )\s*(\d{2,5})/i)
            || line.match(/\b(?:port|on)\s*(\d{2,5})\b/i);

        if (!match) continue;

        const port = parseInt(match[1], 10);
        if (port >= 1024 && port <= 65535) {
            return { url: `/preview/${agentId}/${port}/`, port };
        }
    }

    return null;
}

async function sendPreviewButton(token: string, chatId: number, previewPath: string, port: number, agentId: number): Promise<void> {
    const settings = await import('./db').then(m => m.getAllSettings());
    const publicUrl = settings.public_url;

    if (!publicUrl) return;

    const password = crypto.randomBytes(3).toString('hex');
    setPreviewPassword(agentId, port, password);

    const fullUrl = `${publicUrl}${previewPath}`;
    const keyboard = {
        inline_keyboard: [[
            { text: `🌐 Open Live Preview (Port ${port})`, url: fullUrl }
        ]]
    };

    const url = `https://api.telegram.org/bot${token}/sendMessage`;
    await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            chat_id: chatId,
            text: `🌐 *Web App Published!* (Port ${port})\n\n🔒 *Security Password:* \`${password}\`\n\nClick the link below and enter your password to access the app.`,
            parse_mode: 'Markdown',
            reply_markup: keyboard
        })
    });
}
