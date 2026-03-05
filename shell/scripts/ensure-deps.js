#!/usr/bin/env node
const fs = require('fs');
const { spawnSync } = require('child_process');
const path = require('path');

const shellDir = path.resolve(__dirname, '..');
const tscBin = path.join(shellDir, 'node_modules', '.bin', process.platform === 'win32' ? 'tsc.cmd' : 'tsc');

if (fs.existsSync(tscBin)) {
  process.exit(0);
}

console.log('[bootstrap] Missing local TypeScript toolchain. Installing dependencies with npm ci...');
const result = spawnSync('npm', ['ci'], {
  cwd: shellDir,
  stdio: 'inherit',
  env: process.env
});

if (result.status !== 0) {
  process.exit(result.status || 1);
}
