import { describe, it, expect } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { syncRuntimeFile } from '../src/runtime-assets';

describe('syncRuntimeFile', () => {
  it('copies missing runtime files', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'runtime-assets-'));
    const source = path.join(root, 'source.txt');
    const target = path.join(root, 'target.txt');

    fs.writeFileSync(source, 'new prompt');
    syncRuntimeFile(source, target);

    expect(fs.readFileSync(target, 'utf8')).toBe('new prompt');
  });

  it('overwrites older target files', async () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'runtime-assets-'));
    const source = path.join(root, 'source.txt');
    const target = path.join(root, 'target.txt');

    fs.writeFileSync(source, 'old');
    fs.writeFileSync(target, 'legacy');

    await new Promise((r) => setTimeout(r, 5));
    fs.writeFileSync(source, 'fresh');

    syncRuntimeFile(source, target);

    expect(fs.readFileSync(target, 'utf8')).toBe('fresh');
  });
});
