import { describe, expect, it } from 'vitest';
import fs from 'fs';
import path from 'path';

describe('system_prompt contract examples', () => {
  it('uses XML-tag contract examples instead of JSON envelopes', () => {
    const promptPath = path.join(__dirname, '../../system_prompt.txt');
    const prompt = fs.readFileSync(promptPath, 'utf8');

    expect(prompt).not.toContain('```json');
    expect(prompt).toContain('<thought>');
    expect(prompt).toContain('<message>');
    expect(prompt).toContain('TERMINAL:');
    expect(prompt).toContain('<action>');
    expect(prompt).toContain('Do not emit JSON output.');
    expect(prompt).toContain('<calendar>');
    expect(prompt).toContain('Current System Time (UTC):');
  });
});
