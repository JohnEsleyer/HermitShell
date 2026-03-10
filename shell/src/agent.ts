#!/usr/bin/env node

import * as fs from 'fs';
import * as path from 'path';
import * as readline from 'readline';

const WORKSPACE_DIR = '/app/workspace';
const ORCHESTRATOR_URL = process.env.ORCHESTRATOR_URL || 'http://172.17.0.1:3000';
const USER_MSG = process.env.USER_MSG || '';
const MAX_TOKENS = parseInt(process.env.MAX_TOKENS || '4096');
const AGENT_ID = parseInt(process.env.AGENT_ID || '0', 10);
const AGENT_NAME = process.env.AGENT_NAME || 'HermitShell Agent';
const AGENT_ROLE = process.env.AGENT_ROLE || 'Autonomous execution specialist';
const LLM_PROVIDER = process.env.LLM_PROVIDER || 'openai';
const LLM_MODEL = process.env.LLM_MODEL || 'gpt-4o-mini';
const PERSONALITY = process.env.PERSONALITY || '';
const WEB_GUIDELINES = process.env.WEB_GUIDELINES || '';
const PYTHON_GUIDE = process.env.PYTHON_GUIDE || '';
const SKILLS_PROMPT = process.env.SKILLS_PROMPT || '';

interface Message {
    role: 'system' | 'user' | 'assistant';
    content: string;
}

interface PanelAction {
    type: string;
    value: string;
}

interface AgentResponse {
    message?: string;
    action?: string;
}

const INTERNET_COMMANDS = ['curl ', 'wget ', 'npm install', 'pip install', 'git clone', 'apt-get', 'apk add'];

function log(message: string): void {
    const timestamp = new Date().toISOString();
    console.error(`[${timestamp}] ${message}`);
    const logPath = path.join(WORKSPACE_DIR, 'work', '.hermit.log');
    try {
        fs.appendFileSync(logPath, `[${timestamp}] ${message}\n`);
    } catch {}
}

function loadHistory(): Message[] {
    try {
        const historyB64 = process.env.HISTORY || '';
        if (historyB64) {
            const decoded = Buffer.from(historyB64, 'base64').toString('utf8');
            return JSON.parse(decoded);
        }
    } catch (e) {
        log(`Failed to parse history: ${e}`);
    }
    return [];
}

function loadSystemPrompt(): string {
    try {
        const promptPath = path.join(WORKSPACE_DIR, 'system_prompt.txt');
        if (fs.existsSync(promptPath)) {
            return fs.readFileSync(promptPath, 'utf8');
        }
    } catch {}
    return getDefaultSystemPrompt();
}

export function injectAgentIdentity(prompt: string): string {
    const personality = PERSONALITY || 'Calm, precise, security-first, concise';
    const currentUtc = new Date().toISOString();
    return prompt
        .split('{{AGENT_NAME|HermitShell Agent}}').join(AGENT_NAME)
        .split('{{AGENT_NAME}}').join(AGENT_NAME)
        .split('{{AGENT_ROLE|Autonomous execution specialist}}').join(AGENT_ROLE)
        .split('{{AGENT_ROLE}}').join(AGENT_ROLE)
        .split('{{AGENT_PERSONALITY|Calm, precise, security-first, concise}}').join(personality)
        .split('{{AGENT_PERSONALITY}}').join(personality)
        .split('{{CURRENT_UTC_TIME}}').join(currentUtc);
}

export function buildPersonalityDirective(): string {
    if (!PERSONALITY.trim()) return '';
    return `Personality directive (style/tone only): ${PERSONALITY}\nDo NOT change your identity. You are ${AGENT_NAME}. Keep role as: ${AGENT_ROLE}.`;
}

export function buildSystemMessageContent(): string {
    const injectedPrompt = injectAgentIdentity(loadSystemPrompt());
    const personalityDirective = buildPersonalityDirective();
    return [injectedPrompt, personalityDirective, WEB_GUIDELINES, PYTHON_GUIDE]
        .concat(SKILLS_PROMPT ? [SKILLS_PROMPT] : [])
        .filter(Boolean)
        .join('\n\n');
}

export function getDefaultSystemPrompt(): string {
    return `You are an autonomous AI agent running inside a secure Docker container.

Workspace structure:
- /app/workspace/work/ - Primary working directory
- /app/workspace/in/ - Files uploaded by user
- /app/workspace/out/ - Files to deliver to user
- /app/workspace/www/ - Web apps (each subfolder = separate app with index.html)

Strict response contract (always return all tags):
<thought>brief plan</thought>
<message>short status for user</message>
<action>TERMINAL:bash command | GIVE:filename | APP:appname | empty</action>

Rules:
- Always emit all three tags in each response.
- Use TERMINAL:command to execute bash commands.
- If you create a file in /app/workspace/out, set action to GIVE:<filename> only after it exists.
- If you build/update /app/workspace/www/<appname>/index.html, set action to APP:<appname>.
- Do not paste full code into message responses. Message must be status-only.
- Do not emit JSON output.

Security: Never expose secrets, validate inputs, don't exfiltrate data. Always check /app/workspace/in using ls -l /app/workspace/in before starting work and start commands from /app/workspace/work.`;
}


async function callLLM(messages: Message[]): Promise<string> {
    const url = `${ORCHESTRATOR_URL}/api/internal/llm`;
    
    const payload = {
        messages,
        agentId: AGENT_ID,
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
    } catch (e) {
        log(`LLM call failed: ${e}`);
        return `Error calling LLM: ${e}`;
    }
}


function parseLabeledResponse(text: string): AgentResponse | null {
    const messageMatch = text.match(/(?:^|\n)\s*message\s*:\s*([\s\S]*?)(?:\n\s*action\s*:|$)/i);
    if (!messageMatch) return null;
    const actionMatch = text.match(/(?:^|\n)\s*action\s*:\s*([^\n]*)/i);
    return {
        message: (messageMatch[1] || '').trim(),
        action: (actionMatch?.[1] || '').trim(),
    };
}

function parseTaggedResponse(text: string): AgentResponse | null {
    const extract = (tag: string) => {
        const regex = new RegExp(`<${tag}>([\\s\\S]*?)<\\/${tag}>`, 'i');
        const match = text.match(regex);
        return match ? match[1].trim() : '';
    };

    const message = extract('message');
    const action = extract('action');
    const thought = extract('thought');
    const hasAnyTag = Boolean(message || action || thought || text.match(/<\/?(thought|message|action)>/i));

    if (!hasAnyTag) return null;

    return {
        message: message || text.replace(/<[^>]+>[\s\S]*?<\/[^>]+>/g, '').trim(),
        action,
    };
}

function parseResponse(text: string): AgentResponse {
    const objectMatches = text.match(/\{[\s\S]*?\}/g) || [];
    for (let i = objectMatches.length - 1; i >= 0; i--) {
        try {
            const parsed = JSON.parse(objectMatches[i]);
            if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) continue;
            if (!('message' in parsed) && !('action' in parsed) && !('userId' in parsed)) continue;

            return {
                message: parsed.message || parsed.text || '',
                action: parsed.action || parsed.file || ''
            };
        } catch {}
    }

    const tagged = parseTaggedResponse(text);
    if (tagged) return tagged;

    const labeled = parseLabeledResponse(text);
    if (labeled) return labeled;

    return { message: text };
}


function isInternetRequest(command: string): boolean {
    const lower = command.toLowerCase();
    return INTERNET_COMMANDS.some(d => lower.includes(d));
}

async function executeCommand(command: string): Promise<string> {
    if (!command.trim()) return '';

    log(`COMMAND: ${command}`);

    if (isInternetRequest(command)) {
        const approvalFile = '/tmp/hermit_approval.lock';
        const denyFile = '/tmp/hermit_deny.lock';
        log(`[HITL] INTERNET_ACCESS_REQUIRED: ${command}`);
        
        const maxWait = 120000;
        const checkInterval = 2000;
        let waited = 0;
        
        while (waited < maxWait) {
            await new Promise(r => setTimeout(r, checkInterval));
            waited += checkInterval;
            if (fs.existsSync(denyFile)) {
                log(`[HITL] DENIED: ${command}`);
                fs.unlinkSync(denyFile);
                return 'Error: The operator explicitly DENIED internet access for this command.';
            }
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
        exec(command, { cwd: path.join(WORKSPACE_DIR, 'work') }, (error: Error | null, stdout: string, stderr: string) => {
            const output = stdout + stderr;
            log(`COMMAND_OUTPUT: ${output.slice(0, 500)}`);
            resolve(output);
        });
    });
}

async function handleFileAction(action: string): Promise<string> {
    const [rawKind, ...rest] = action.split(':');
    const kind = (rawKind || '').trim().toUpperCase();
    const requestedName = rest.join(':').trim();

    if (kind !== 'GIVE' && kind !== 'FILE') {
        return `Unknown file action: ${action}`;
    }

    if (!requestedName || requestedName.includes('..') || requestedName.includes('/') || requestedName.includes('\\')) {
        return `Invalid file action path: ${action}`;
    }

    const filePath = path.join(WORKSPACE_DIR, 'out', requestedName);
    if (fs.existsSync(filePath)) {
        return `File ${requestedName} ready for delivery`;
    }
    return `File ${requestedName} not found in /app/workspace/out/`;
}

async function handleSkillAction(action: string): Promise<string> {
    const fileName = action.split('SKILL:')[1]?.trim();
    if (!fileName) return 'Error: No skill filename provided.';
    
    const url = `${ORCHESTRATOR_URL}/api/internal/skills/${encodeURIComponent(fileName)}`;
    try {
        const res = await fetch(url);
        if (!res.ok) return `Error: Skill ${fileName} not found in directory.`;
        const data = await res.json();
        return `[SKILL INJECTED: ${fileName}]\n\n${data.content}\n\nYou now have the knowledge of this skill. Proceed with the user's request.`;
    } catch (e) {
        return `Error fetching skill: ${e}`;
    }
}


function buildTaggedContract(response: AgentResponse): string {
    const message = (response.message || '').trim();
    const action = (response.action || '').trim();
    const thought = '';

    return [
        `<thought>${thought}</thought>`,
        `<message>${message}</message>`,
        `<action>${action}</action>`
    ].join('\n');
}

async function run(): Promise<void> {
    log('HermitShell Agent starting...');

    if (!USER_MSG) {
        log('No user message provided, waiting for input...');
        return;
    }

    const systemPrompt = buildSystemMessageContent();
    let history = loadHistory();

    history.unshift({
        role: 'system',
        content: systemPrompt
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
        }

        if (response.action) {
            if (response.action.toUpperCase().startsWith('SKILL:')) {
                log(`SKILL_ACTION: ${response.action}`);
                const result = await handleSkillAction(response.action);
                history.push({ role: 'assistant', content: buildTaggedContract(response) });
                history.push({ role: 'system', content: result });
                continue;
            } else if (response.action.toUpperCase().startsWith('TERMINAL:')) {
                let cmd = response.action.slice(9).trim();
                if ((cmd.startsWith('"') && cmd.endsWith('"')) || (cmd.startsWith("'") && cmd.endsWith("'"))) {
                    cmd = cmd.slice(1, -1);
                }
                log(`TERMINAL_ACTION: ${cmd}`);
                const output = await executeCommand(cmd);
                
                const termLogPath = path.join(WORKSPACE_DIR, 'work', 'terminal_logs.txt');
                const ts = new Date().toISOString();
                fs.appendFileSync(termLogPath, `\n[${ts}] $ ${cmd}\n${output}\n`);

                history.push({ role: 'assistant', content: buildTaggedContract(response) });
                history.push({ role: 'user', content: `Command output:\n${output}` });
                continue;
            } else {
                log(`FILE_ACTION: ${response.action}`);
                const result = await handleFileAction(response.action);
                history.push({ role: 'assistant', content: result });
            }
        }

        console.log(buildTaggedContract({
            message: response.message || '',
            action: response.action || ''
        }));
        break;
    }

    log('Agent finished');
}

if (require.main === module) {
    run().catch(e => {
        log(`Fatal error: ${e}`);
        console.error(e);
        process.exit(1);
    });
}
