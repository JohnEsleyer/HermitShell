export type ParsedAgentResponse = {
    userId?: string;
    message: string;
    action: string;
    terminal: string;
    panelActions: string[];
};

function asString(value: unknown): string {
    if (typeof value === 'string') return value;
    if (value === null || value === undefined) return '';
    return String(value);
}

function tryParseContractJson(candidate: string): ParsedAgentResponse | null {
    try {
        const parsed = JSON.parse(candidate);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;

        const hasContractField = (
            Object.prototype.hasOwnProperty.call(parsed, 'message') ||
            Object.prototype.hasOwnProperty.call(parsed, 'terminal') ||
            Object.prototype.hasOwnProperty.call(parsed, 'action') ||
            Object.prototype.hasOwnProperty.call(parsed, 'userId')
        );

        if (!hasContractField) return null;

        const panelActions: string[] = [];

        return {
            userId: (parsed as any).userId !== undefined ? asString((parsed as any).userId) : undefined,
            message: asString((parsed as any).message).trim(),
            action: asString((parsed as any).action).trim(),
            terminal: asString((parsed as any).terminal).trim(),
            panelActions
        };
    } catch {
        return null;
    }
}

export function parseAgentResponse(rawOutput: string): ParsedAgentResponse {
    const fallback: ParsedAgentResponse = {
        message: asString(rawOutput).trim(),
        action: '',
        terminal: '',
        panelActions: []
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

    return fallback;
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
