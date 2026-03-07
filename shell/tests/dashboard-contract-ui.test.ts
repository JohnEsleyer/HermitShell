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
    expect(html).not.toContain('✅ JSON parsed');
    expect(html).toContain('/xml-contract');
    expect(html).not.toContain('/json-contract');
  });
});
