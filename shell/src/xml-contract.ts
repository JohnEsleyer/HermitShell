function escapeXml(value: string): string {
    return String(value || '')
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

export function buildXmlContract(input: {
    thought?: string;
    message?: string;
    action?: string;
}): string {
    const thought = String(input.thought || '').trim();
    const message = String(input.message || '').trim();
    const action = String(input.action || '').trim();

    const chunks: string[] = [];
    if (thought) chunks.push(`<thought>${escapeXml(thought)}</thought>`);
    chunks.push(`<message>${escapeXml(message)}</message>`);
    chunks.push(`<action>${escapeXml(action)}</action>`);
    return chunks.join('\n');
}
