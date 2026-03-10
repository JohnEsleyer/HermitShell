import { describe, expect, it } from 'vitest';
import { detectContractFormat, hasStructuredContract, normalizeAgentOutputToJson, parseAgentResponse, parseFileAction, parseTerminalAction, toContractJson } from '../src/agent-response';

describe('parseAgentResponse', () => {
  it('parses deterministic json payload', () => {
    const result = parseAgentResponse('{"userId":"100","message":"Done","action":"TERMINAL:python run.py"}');
    expect(result.userId).toBe('100');
    expect(result.message).toBe('Done');
    expect(result.action).toBe('TERMINAL:python run.py');
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
      '{"userId":"123","message":"Hello there","action":""}',
      '2026-03-05T09:25:23.187Z Agent finished'
    ].join('\n');

    const result = parseAgentResponse(output);
    expect(result.userId).toBe('123');
    expect(result.message).toBe('Hello there');
    expect(result.action).toBe('');
  });


  it('does not treat non-contract JSON payloads as contract output', () => {
    const raw = '{"foo":"bar"}';
    const result = parseAgentResponse(raw);
    expect(result.message).toBe(raw);
    expect(result.action).toBe('');
  });


  it('extracts inline GIVE action from plain text fallback output', () => {
    const result = parseAgentResponse('Done. GIVE:hello.txt');
    expect(result.message).toBe('Done. GIVE:hello.txt');
    expect(result.action).toBe('GIVE:hello.txt');
  });

  it('extracts inline APP action from plain text fallback output', () => {
    const result = parseAgentResponse('App deployed at endpoint APP:todo-ui');
    expect(result.action).toBe('APP:todo-ui');
  });



  it('parses xml-style tagged contract fields', () => {
    const result = parseAgentResponse(`<thought>plan</thought>
<message>Created file</message>
<action>TERMINAL:echo hi > /app/workspace/out/a.txt</action>`);
    expect(result.message).toBe('Created file');
    expect(result.action).toBe('TERMINAL:echo hi > /app/workspace/out/a.txt');
  });

  it('marks tagged XML contracts as structured', () => {
    const payload = `<message>Done</message><action></action>`;
    expect(hasStructuredContract(payload)).toBe(true);
    expect(detectContractFormat(payload)).toBe('xml');
  });

  it('detects JSON contract format', () => {
    const payload = '{"message":"Done","action":""}';
    expect(detectContractFormat(payload)).toBe('json');
  });

  it('parses labeled contract fields without JSON envelope', () => {
    const result = parseAgentResponse(`message: Created file\naction: TERMINAL:echo hi > /app/workspace/out/a.txt`);
    expect(result.message).toBe('Created file');
    expect(result.action).toBe('TERMINAL:echo hi > /app/workspace/out/a.txt');
  });

  it('uses the latest valid JSON object in output', () => {
    const output = [
      '{"message":"first","action":""}',
      'log line',
      '{"message":"second","action":"GIVE:hello.txt"}'
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

describe('parseTerminalAction', () => {
  it('extracts terminal command from action', () => {
    expect(parseTerminalAction('TERMINAL:ls -la')).toBe('ls -la');
    expect(parseTerminalAction('TERMINAL:"ls -la"')).toBe('ls -la');
    expect(parseTerminalAction("TERMINAL:'ls -la'")).toBe('ls -la');
    expect(parseTerminalAction('GIVE:file.txt')).toBeNull();
    expect(parseTerminalAction('APP:todo')).toBeNull();
  });
});

describe('JSON normalization helpers', () => {
  it('normalizes XML contract output into canonical JSON log payload', () => {
    const raw = '<message>I created a file for you</message> this text is ignored <action>GIVE:filename.txt</action>';
    const normalized = normalizeAgentOutputToJson(raw, 6585507149);
    expect(normalized).toBe('{"message":"I created a file for you","action":"GIVE:filename.txt","userId":"6585507149"}');
  });

  it('serializes parsed contracts with stable JSON keys', () => {
    const payload = toContractJson({
      userId: '123',
      message: 'Done',
      action: 'APP:todo',
    });
    expect(payload).toBe('{"message":"Done","action":"APP:todo","userId":"123"}');
  });
});
