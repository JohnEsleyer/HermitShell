export type ParsedAgentResponse = {
    userId?: string;
    message: string;
    action: string;
};

export interface CalendarItem {
    datetime: string;
    prompt: string;
}

function asString(value: unknown): string {
    if (typeof value === 'string') return value;
    if (value === null || value === undefined) return '';
    return String(value);
}

export function extractCalendars(text: string): CalendarItem[] {
    const results: CalendarItem[] = [];
    const textStr = asString(text);
    const regex = /<calendar>\s*<datetime>(.*?)<\/datetime>\s*<prompt>([\s\S]*?)<\/prompt>\s*<\/calendar>/gi;
    let match;
    while ((match = regex.exec(textStr)) !== null) {
        results.push({
            datetime: match[1].trim(),
            prompt: match[2].trim()
        });
    }
    return results;
}

function parseTaggedContract(rawOutput: string): ParsedAgentResponse | null {
    const text = asString(rawOutput);
    const extract = (tag: string): string => {
        const regex = new RegExp(`<${tag}>([\\s\\S]*?)<\\/${tag}>`, 'i');
        const match = text.match(regex);
        return match ? asString(match[1]).trim() : '';
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

function parseLabeledContract(rawOutput: string): ParsedAgentResponse | null {
    const text = asString(rawOutput);
    const match = text.match(/(?:^|\n)\s*message\s*:\s*([\s\S]*?)(?:\n\s*action\s*:|$)/i);
    if (!match) return null;

    const actionMatch = text.match(/(?:^|\n)\s*action\s*:\s*([^\n]*)/i);

    return {
        message: asString(match[1]).trim(),
        action: asString(actionMatch?.[1]).trim(),
    };
}

function tryParseContractJson(candidate: string): ParsedAgentResponse | null {
    try {
        const parsed = JSON.parse(candidate);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;

        const hasContractField = (
            Object.prototype.hasOwnProperty.call(parsed, 'message') ||
            Object.prototype.hasOwnProperty.call(parsed, 'action') ||
            Object.prototype.hasOwnProperty.call(parsed, 'userId')
        );

        if (!hasContractField) return null;

        return {
            userId: (parsed as any).userId !== undefined ? asString((parsed as any).userId) : undefined,
            message: asString((parsed as any).message).trim(),
            action: asString((parsed as any).action).trim(),
        };
    } catch {
        return null;
    }
}

export function detectContractFormat(rawOutput: string): 'xml' | 'json' | 'labeled' | 'none' {
    const output = asString(rawOutput);

    const objectMatches = output.match(/\{[\s\S]*?\}/g) || [];
    for (let i = objectMatches.length - 1; i >= 0; i--) {
        if (tryParseContractJson(objectMatches[i])) return 'json';
    }

    const firstBrace = output.indexOf('{');
    const lastBrace = output.lastIndexOf('}');
    if (firstBrace !== -1 && lastBrace !== -1 && lastBrace > firstBrace) {
        if (tryParseContractJson(output.substring(firstBrace, lastBrace + 1))) return 'json';
    }

    if (parseTaggedContract(output)) return 'xml';
    if (parseLabeledContract(output)) return 'labeled';
    return 'none';
}

export function hasStructuredContract(rawOutput: string): boolean {
    return detectContractFormat(rawOutput) !== 'none';
}

function extractInlineAction(text: string): string {
    const content = asString(text);
    if (!content.trim()) return '';

    const lines = content.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    const lineMatch = [...lines].reverse().find((line) => /^(GIVE|FILE|APP|TERMINAL|SKILL)\s*:/i.test(line));
    if (lineMatch) {
        return lineMatch.replace(/^['"`]|['"`]$/g, '').trim();
    }

    const inline = content.match(/\b(GIVE|FILE|APP|TERMINAL|SKILL)\s*:\s*([^\s'"`]+)/i);
    if (inline) {
        return `${inline[1].toUpperCase()}:${inline[2].trim()}`;
    }

    return '';
}

export function parseAgentResponse(rawOutput: string): ParsedAgentResponse {
    const fallback: ParsedAgentResponse = {
        message: asString(rawOutput).trim(),
        action: '',
    };

    const output = asString(rawOutput);

    const objectMatches = output.match(/\{[\s\S]*?\}/g) || [];
    for (let i = objectMatches.length - 1; i >= 0; i--) {
        const parsed = tryParseContractJson(objectMatches[i]);
        if (parsed) return parsed;
    }

    const firstBrace = output.indexOf('{');
    const lastBrace = output.lastIndexOf('}');
    if (firstBrace !== -1 && lastBrace !== -1 && lastBrace > firstBrace) {
        const parsed = tryParseContractJson(output.substring(firstBrace, lastBrace + 1));
        if (parsed) return parsed;
    }

    const tagged = parseTaggedContract(output);
    if (tagged) return tagged;

    const labeled = parseLabeledContract(output);
    if (labeled) return labeled;

    const inferredAction = extractInlineAction(output);
    if (inferredAction) {
        return {
            ...fallback,
            action: inferredAction
        };
    }

    return fallback;
}


export function toContractJson(parsed: ParsedAgentResponse, userId?: string | number): string {
    const payload: Record<string, string> = {
        message: asString(parsed.message).trim(),
        action: asString(parsed.action).trim()
    };

    const resolvedUserId = userId !== undefined && userId !== null
        ? asString(userId).trim()
        : asString(parsed.userId).trim();

    if (resolvedUserId) {
        payload.userId = resolvedUserId;
    }

    return JSON.stringify(payload);
}

export function normalizeAgentOutputToJson(rawOutput: string, userId?: string | number): string {
    const parsed = parseAgentResponse(rawOutput);
    return toContractJson(parsed, userId);
}

export function parseFileAction(action: string): string | null {
    const normalized = asString(action).trim();
    const upper = normalized.toUpperCase();
    if (!upper.startsWith('GIVE:') && !upper.startsWith('FILE:')) return null;

    const candidate = normalized.slice(normalized.indexOf(':') + 1).trim();
    if (!candidate) return null;

    if (candidate.includes('..') || candidate.includes('/') || candidate.includes('\\')) {
        return null;
    }

    return candidate;
}

export function parseAppAction(action: string): string | null {
    const normalized = asString(action).trim();
    if (!normalized.toUpperCase().startsWith('APP:')) return null;

    const appName = normalized.slice(4).trim();
    if (!appName) return null;
    if (appName.includes('..') || appName.includes('/') || appName.includes('\\')) return null;
    return appName;
}

export function parseSkillAction(action: string): string | null {
    const normalized = asString(action).trim();
    if (!normalized.toUpperCase().startsWith('SKILL:')) return null;

    const skillName = normalized.slice(6).trim();
    if (!skillName || skillName.includes('..') || skillName.includes('/') || skillName.includes('\\')) {
        return null;
    }
    return skillName;
}

export function parseTerminalAction(action: string): string | null {
    const normalized = asString(action).trim();
    if (!normalized.toUpperCase().startsWith('TERMINAL:')) return null;
    let cmd = normalized.slice(9).trim();
    if ((cmd.startsWith('"') && cmd.endsWith('"')) || (cmd.startsWith("'") && cmd.endsWith("'"))) {
        cmd = cmd.slice(1, -1);
    }
    return cmd;
}
