import { describe, expect, it } from 'vitest';
import { parseAgentResponse, parseFileAction } from '../src/agent-response';

describe('parseAgentResponse', () => {
  it('parses deterministic json payload', () => {
    const result = parseAgentResponse('{"userId":"100","message":"Done","action":"FILE:report.pdf","terminal":"python run.py"}');
    expect(result.userId).toBe('100');
    expect(result.message).toBe('Done');
    expect(result.action).toBe('FILE:report.pdf');
    expect(result.terminal).toBe('python run.py');
  });

  it('falls back to raw output when no json exists', () => {
    const result = parseAgentResponse('Plain response');
    expect(result.message).toBe('Plain response');
    expect(result.action).toBe('');
  });

  it('extracts contract JSON from noisy logs', () => {
    const output = [
      '2026-03-05T09:25:17.114Z HermitShell Agent starting...',
      '2026-03-05T09:25:17.123Z Iteration 1/10',
      '{"userId":"123","message":"Hello there","terminal":"","action":""}',
      '2026-03-05T09:25:23.187Z Agent finished'
    ].join('\n');

    const result = parseAgentResponse(output);
    expect(result.userId).toBe('123');
    expect(result.message).toBe('Hello there');
    expect(result.terminal).toBe('');
    expect(result.action).toBe('');
  });

  it('uses the latest valid JSON object in output', () => {
    const output = [
      '{"message":"first","action":"","terminal":""}',
      'log line',
      '{"message":"second","action":"GIVE:hello.txt","terminal":""}'
    ].join('\n');

    const result = parseAgentResponse(output);
    expect(result.message).toBe('second');
    expect(result.action).toBe('GIVE:hello.txt');
  });
});

describe('parseFileAction', () => {
  it('returns only safe file names', () => {
    expect(parseFileAction('FILE:invoice.txt')).toBe('invoice.txt');
    expect(parseFileAction('FILE:../secrets.txt')).toBeNull();
    expect(parseFileAction('')).toBeNull();
  });
});
