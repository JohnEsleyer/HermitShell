#!/usr/bin/env node
const fs = require('fs');
const { spawnSync } = require('child_process');
const path = require('path');

const shellDir = path.resolve(__dirname, '..');
const distServer = path.join(shellDir, 'dist', 'server.js');

if (!fs.existsSync(distServer)) {
  console.log('[bootstrap] dist/server.js not found. Running npm run build...');
  const build = spawnSync('npm', ['run', 'build'], {
    cwd: shellDir,
    stdio: 'inherit',
    env: process.env
  });
  if (build.status !== 0) {
    process.exit(build.status || 1);
  }
}

const run = spawnSync('node', [distServer], {
  cwd: shellDir,
  stdio: 'inherit',
  env: process.env
});

process.exit(run.status || 0);
