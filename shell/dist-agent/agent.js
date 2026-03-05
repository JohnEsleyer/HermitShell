#!/usr/bin/env node
"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const WORKSPACE_DIR = '/app/workspace';
const ORCHESTRATOR_URL = process.env.ORCHESTRATOR_URL || 'http://172.17.0.1:3000';
const USER_MSG = process.env.USER_MSG || '';
const MAX_TOKENS = parseInt(process.env.MAX_TOKENS || '4096');
const LLM_PROVIDER = process.env.LLM_PROVIDER || 'openai';
const LLM_MODEL = process.env.LLM_MODEL || 'gpt-4o-mini';
const PERSONALITY = process.env.PERSONALITY || '';
const WEB_GUIDELINES = process.env.WEB_GUIDELINES || '';
const PYTHON_GUIDE = process.env.PYTHON_GUIDE || '';
const DANGEROUS_COMMANDS = ['rm -rf', 'sudo', 'docker', 'chmod 777', 'mkfs', 'dd if='];
function log(message) {
    const timestamp = new Date().toISOString();
    console.log(`[${timestamp}] ${message}`);
    const logPath = path.join(WORKSPACE_DIR, 'work', '.hermit.log');
    try {
        fs.appendFileSync(logPath, `[${timestamp}] ${message}\n`);
    }
    catch { }
}
function loadHistory() {
    try {
        const historyB64 = process.env.HISTORY || '';
        if (historyB64) {
            const decoded = Buffer.from(historyB64, 'base64').toString('utf8');
            return JSON.parse(decoded);
        }
    }
    catch (e) {
        log(`Failed to parse history: ${e}`);
    }
    return [];
}
function loadSystemPrompt() {
    try {
        const promptPath = path.join(WORKSPACE_DIR, 'system_prompt.txt');
        if (fs.existsSync(promptPath)) {
            return fs.readFileSync(promptPath, 'utf8');
        }
    }
    catch { }
    return getDefaultSystemPrompt();
}
function getDefaultSystemPrompt() {
    return `You are an autonomous AI agent running inside a secure Docker container.

Workspace structure:
- /app/workspace/work/ - Primary working directory
- /app/workspace/in/ - Files uploaded by user
- /app/workspace/out/ - Files to deliver to user
- /app/workspace/www/ - Web apps (each subfolder = separate app with index.html)

Available actions (return JSON):
{
  "message": "response to user",
  "terminal": "bash command",
  "action": "GIVE:filename"
}

The legacy \`panelActions\` field is deprecated and should not be used.

Security: Never expose secrets, validate inputs, don't exfiltrate data. Always check /app/workspace/in using ls -l /app/workspace/in before starting work and start commands from /app/workspace/work.`;
}
async function callLLM(messages) {
    const url = `${ORCHESTRATOR_URL}/api/internal/llm`;
    const payload = {
        messages,
        model: LLM_MODEL,
        provider: LLM_PROVIDER,
        max_tokens: MAX_TOKENS,
        temperature: 0.7
    };
    log(`Calling LLM at ${url}`);
    try {
        const response = await fetch(url, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!response.ok) {
            throw new Error(`LLM request failed: ${response.status}`);
        }
        const data = await response.json();
        return data.output || data.content || data.message || '';
    }
    catch (e) {
        log(`LLM call failed: ${e}`);
        return `Error calling LLM: ${e}`;
    }
}
function parseResponse(text) {
    const jsonMatch = text.match(/\{[\s\S]*\}/);
    if (jsonMatch) {
        try {
            const parsed = JSON.parse(jsonMatch[0]);
            return {
                message: parsed.message || parsed.text || '',
                terminal: parsed.terminal || parsed.command || '',
                panelActions: parsed.panelActions || parsed.actions || [],
                action: parsed.action || parsed.file || ''
            };
        }
        catch { }
    }
    return { message: text };
}
function isDangerous(command) {
    const lower = command.toLowerCase();
    return DANGEROUS_COMMANDS.some(d => lower.includes(d.toLowerCase()));
}
async function executeCommand(command) {
    if (!command.trim())
        return '';
    log(`COMMAND: ${command}`);
    if (isDangerous(command)) {
        const approvalFile = '/tmp/hermit_approval.lock';
        log(`[HITL] APPROVAL_REQUIRED: ${command}`);
        const maxWait = 120000;
        const checkInterval = 2000;
        let waited = 0;
        while (waited < maxWait) {
            await new Promise(r => setTimeout(r, checkInterval));
            waited += checkInterval;
            if (fs.existsSync(approvalFile)) {
                log(`[HITL] APPROVED: ${command}`);
                fs.unlinkSync(approvalFile);
                break;
            }
        }
        if (waited >= maxWait) {
            log(`[HITL] TIMEOUT: Command not approved`);
            return 'Command required approval and timed out waiting.';
        }
    }
    return new Promise((resolve) => {
        const { exec } = require('child_process');
        exec(command, { cwd: path.join(WORKSPACE_DIR, 'work') }, (error, stdout, stderr) => {
            const output = stdout + stderr;
            log(`COMMAND_OUTPUT: ${output.slice(0, 500)}`);
            resolve(output);
        });
    });
}
function parsePanelAction(action) {
    const parts = action.split(':');
    if (parts.length >= 2) {
        return { type: parts[0], value: parts.slice(1).join(':') };
    }
    return null;
}
async function handleCalendarAction(action) {
    const parsed = parsePanelAction(action);
    if (!parsed)
        return '';
    const parts = parsed.value.split('|');
    const calendarDbPath = path.join(WORKSPACE_DIR, 'work', 'calendar.db');
    try {
        let createClient;
        try {
            ({ createClient } = await Promise.resolve().then(() => __importStar(require('@libsql/client'))));
        }
        catch {
            log('Calendar action skipped: @libsql/client not installed in runtime');
            return 'Calendar actions are unavailable in this runtime.';
        }
        const db = createClient({ url: `file:${calendarDbPath}` });
        await db.execute(`
            CREATE TABLE IF NOT EXISTS calendar_events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                title TEXT NOT NULL,
                prompt TEXT NOT NULL,
                start_time TEXT NOT NULL,
                end_time TEXT,
                target_user_id INTEGER,
                status TEXT DEFAULT 'scheduled'
            )
        `);
        switch (parsed.type) {
            case 'CALENDAR_CREATE': {
                const [title, prompt, startTime, endTime] = parts;
                await db.execute({
                    sql: `INSERT INTO calendar_events (title, prompt, start_time, end_time, status) VALUES (?, ?, ?, ?, 'scheduled')`,
                    args: [title, prompt, startTime || '', endTime || '']
                });
                return `Calendar event "${title}" scheduled for ${startTime}`;
            }
            case 'CALENDAR_LIST': {
                const rs = await db.execute({ sql: 'SELECT * FROM calendar_events ORDER BY start_time ASC', args: [] });
                return `Calendar events: ${JSON.stringify(rs.rows)}`;
            }
            case 'CALENDAR_DELETE': {
                const eventId = parts[0];
                await db.execute({ sql: 'DELETE FROM calendar_events WHERE id = ?', args: [eventId] });
                return `Calendar event ${eventId} deleted`;
            }
            default:
                return `Unknown calendar action: ${parsed.type}`;
        }
    }
    catch (e) {
        log(`Calendar action error: ${e}`);
        return `Calendar action failed: ${e}`;
    }
}
async function handleFileAction(action) {
    const filePath = path.join(WORKSPACE_DIR, 'out', action);
    if (fs.existsSync(filePath)) {
        return `File ${action} ready for delivery`;
    }
    return `File ${action} not found in /app/workspace/out/`;
}
async function run() {
    log('HermitShell Agent starting...');
    if (!USER_MSG) {
        log('No user message provided, waiting for input...');
        return;
    }
    const systemPrompt = loadSystemPrompt();
    let history = loadHistory();
    history.unshift({
        role: 'system',
        content: systemPrompt + '\n\n' + PERSONALITY + '\n\n' + WEB_GUIDELINES + '\n\n' + PYTHON_GUIDE
    });
    history.push({ role: 'user', content: USER_MSG });
    const maxIterations = 10;
    let iterations = 0;
    while (iterations < maxIterations) {
        iterations++;
        log(`Iteration ${iterations}/${maxIterations}`);
        const llmResponse = await callLLM(history);
        const response = parseResponse(llmResponse);
        if (response.message) {
            log(`RESPONSE: ${response.message}`);
            console.log(response.message);
        }
        if (response.panelActions) {
            for (const action of response.panelActions) {
                log(`ACTION: ${action}`);
                if (action.startsWith('CALENDAR_')) {
                    const result = await handleCalendarAction(action);
                    history.push({ role: 'assistant', content: result });
                }
            }
        }
        if (response.action) {
            log(`FILE_ACTION: ${response.action}`);
            const result = await handleFileAction(response.action);
            history.push({ role: 'assistant', content: result });
        }
        if (!response.terminal) {
            if (response.message) {
                console.log(`\n${response.message}`);
            }
            break;
        }
        const output = await executeCommand(response.terminal);
        history.push({ role: 'user', content: `Command output:\n${output}` });
    }
    log('Agent finished');
}
run().catch(e => {
    log(`Fatal error: ${e}`);
    console.error(e);
    process.exit(1);
});
