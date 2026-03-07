import { describe, expect, it, beforeEach, afterEach } from 'vitest';

const ORIGINAL_ENV = { ...process.env };

async function loadAgentModule() {
  return import('../src/agent');
}

describe('agent system prompt identity injection', () => {
  beforeEach(() => {
    process.env.AGENT_NAME = 'Rain';
    process.env.AGENT_ROLE = 'Assistant';
    process.env.PERSONALITY = 'Act like Hu Tao';
  });

  afterEach(() => {
    process.env = { ...ORIGINAL_ENV };
  });

  it('injects AGENT_NAME placeholders', async () => {
    const { injectAgentIdentity } = await loadAgentModule();
    const prompt = 'You are {{AGENT_NAME|HermitShell Agent}} / {{AGENT_NAME}}';
    const output = injectAgentIdentity(prompt);
    expect(output).toContain('Rain');
    expect(output).not.toContain('{{AGENT_NAME');
  });

  it('keeps personality as style directive and preserves identity', async () => {
    const { buildPersonalityDirective } = await loadAgentModule();
    const output = buildPersonalityDirective();
    expect(output).toContain('style/tone only');
    expect(output).toContain('You are Rain');
    expect(output).toContain('Act like Hu Tao');
  });


  it('default prompt enforces XML-tag contract (not JSON)', async () => {
    const { getDefaultSystemPrompt } = await loadAgentModule();
    const output = getDefaultSystemPrompt();
    expect(output).toContain('<thought>');
    expect(output).toContain('<message>');
    expect(output).toContain('<terminal>');
    expect(output).toContain('<action>');
    expect(output).toContain('Do not emit JSON output.');
  });

});
