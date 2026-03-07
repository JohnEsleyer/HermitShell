import { describe, it, expect } from 'vitest';
import fs from 'fs';
import path from 'path';

function read(rel: string): string {
  return fs.readFileSync(path.join(__dirname, '../..', rel), 'utf8');
}

describe('dashboard contract UI copy and endpoints', () => {
  const files = ['dashboard/src/public/index.html', 'dashboard/public/index.html'];

  it.each(files)('%s uses XML contract wording in agent testing UX', (file) => {
    const html = read(file);
    expect(html).toContain('Respond with your status and current capabilities using XML tags');
    expect(html).not.toContain('current capabilities in JSON');
    expect(html).toContain('✅ ${String(entry.parsed?.contractFormat || \'structured\').toUpperCase()} parsed');
    expect(html).toContain('/xml-contract');
    expect(html).not.toContain('/json-contract');
  });

  it.each(files)('%s keeps workspace modal above sidebar layers', (file) => {
    const html = read(file);
    expect(html).toContain('dashboard-sidebar glass-panel dashboard-sidebar flex-col z-20 hidden xl:flex');
    expect(html).toContain('<main class="dashboard-main relative overflow-hidden">');
    expect(html).toContain('id="workspace-modal" class="fixed inset-0 bg-black/70 z-[120] hidden items-center justify-center p-4"');
  });

  it.each(files)('%s shows XML-vs-JSON explainer controls in agent test modal', (file) => {
    const html = read(file);
    expect(html).toContain('toggleAgentContractHelp()');
    expect(html).toContain('id="agent-contract-help"');
    expect(html).toContain('Agents emit XML. HermitShell parses tags and stores normalized JSON logs.');
    expect(html).toContain('<strong>Agent output format:</strong> XML tags');
    expect(html).toContain('<strong>System log format:</strong> JSON objects');
  });

  it.each(files)('%s keeps left logs JSON and right input XML in Agent Test modal', (file) => {
    const html = read(file);
    expect(html).toContain("✅ ${String(entry.parsed?.contractFormat || 'structured').toUpperCase()} parsed");
    expect(html).toContain('XML Contract Input');
    expect(html).toContain('Run XML Contract Test');
    expect(html).toContain('✅ JSON parsed');
  });

});
