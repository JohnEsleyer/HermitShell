#!/usr/bin/env node
const { spawnSync } = require('child_process');
const path = require('path');

const shellDir = path.resolve(__dirname, '..');
const distServer = path.join(shellDir, 'dist', 'server.js');
const skipBuild = process.env.HERMITSHELL_SKIP_BUILD === '1';

if (!skipBuild) {
  console.log('[bootstrap] Running npm run build to ensure latest shell + dashboard changes are applied...');
  const build = spawnSync('npm', ['run', 'build'], {
    cwd: shellDir,
    stdio: 'inherit',
    env: process.env
  });
  if (build.status !== 0) {
    process.exit(build.status || 1);
  }
} else {
  console.log('[bootstrap] HERMITSHELL_SKIP_BUILD=1 set. Skipping build and starting existing dist artifacts.');
}

const run = spawnSync('node', [distServer], {
  cwd: shellDir,
  stdio: 'inherit',
  env: process.env
});

process.exit(run.status || 0);
