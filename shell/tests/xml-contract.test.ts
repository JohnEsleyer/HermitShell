import { describe, it, expect } from 'vitest';
import { buildXmlContract } from '../src/xml-contract';
import { detectContractFormat, parseAgentResponse } from '../src/agent-response';

describe('buildXmlContract', () => {
  it('builds deterministic xml with canonical fields', () => {
    const xml = buildXmlContract({
      message: 'Delivered report',
      action: 'GIVE:report.txt'
    });

    expect(xml).toContain('<message>Delivered report</message>');
    expect(xml).toContain('<action>GIVE:report.txt</action>');
    expect(detectContractFormat(xml)).toBe('xml');

    const parsed = parseAgentResponse(xml);
    expect(parsed.message).toBe('Delivered report');
    expect(parsed.action).toBe('GIVE:report.txt');
  });

  it('escapes xml entities in fields', () => {
    const xml = buildXmlContract({
      message: 'A & B < C > D',
      action: 'TERMINAL:echo "1 < 2"'
    });

    expect(xml).toContain('<message>A &amp; B &lt; C &gt; D</message>');
    expect(xml).toContain('<action>TERMINAL:echo "1 &lt; 2"</action>');
  });
});
