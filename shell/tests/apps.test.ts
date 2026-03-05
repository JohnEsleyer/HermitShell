import { describe, it, expect, beforeEach } from 'vitest';
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';
import { discoverSitesFromWorkspaces, deleteWebApp } from '../src/sites';

describe('App Discovery with index.html', () => {
  it('only discovers apps with index.html', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'hermit-apps-'));
    fs.mkdirSync(path.join(root, '1_10', 'www', 'calculator'), { recursive: true });
    fs.mkdirSync(path.join(root, '1_10', 'www', 'empty-app'), { recursive: true });
    fs.writeFileSync(path.join(root, '1_10', 'www', 'calculator', 'index.html'), '<h1>Calculator</h1>');

    const sites = discoverSitesFromWorkspaces(
      root,
      [{ id: 1, name: 'TestAgent', docker_image: 'test:latest' }],
      'http://localhost:3000',
      () => null
    );

    expect(sites).toHaveLength(1);
    expect(sites[0].webApps).toHaveLength(1);
    expect(sites[0].webApps[0].siteName).toBe('calculator');
    expect(sites[0].webApps[0].hasIndexHtml).toBe(true);

    fs.rmSync(root, { recursive: true, force: true });
  });

  it('only includes apps with index.html in webApps list', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'hermit-apps-'));
    fs.mkdirSync(path.join(root, '1_10', 'www', 'no-index'), { recursive: true });
    fs.writeFileSync(path.join(root, '1_10', 'www', 'no-index', 'styles.css'), 'body {}');

    const sites = discoverSitesFromWorkspaces(
      root,
      [{ id: 1, name: 'TestAgent', docker_image: 'test:latest' }],
      'http://localhost:3000',
      () => null
    );

    expect(sites).toHaveLength(1);
    expect(sites[0].webApps).toHaveLength(0);

    fs.rmSync(root, { recursive: true, force: true });
  });
});

describe('WebApp Type', () => {
  it('includes isActive field', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'hermit-apps-'));
    fs.mkdirSync(path.join(root, '1_10', 'www', 'testapp'), { recursive: true });
    fs.writeFileSync(path.join(root, '1_10', 'www', 'testapp', 'index.html'), '<h1>Test</h1>');

    const sites = discoverSitesFromWorkspaces(
      root,
      [{ id: 1, name: 'TestAgent', docker_image: 'test:latest' }],
      'http://localhost:3000',
      () => null
    );

    expect(sites[0].webApps[0]).toHaveProperty('isActive');
    expect(sites[0].webApps[0].isActive).toBe(false);

    fs.rmSync(root, { recursive: true, force: true });
  });
});

describe('deleteWebApp', () => {
  it('deletes app folder with all files', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'hermit-apps-'));
    const appPath = path.join(root, '1_10', 'www', 'myapp');
    fs.mkdirSync(appPath, { recursive: true });
    fs.writeFileSync(path.join(appPath, 'index.html'), '<h1>App</h1>');
    fs.writeFileSync(path.join(appPath, 'style.css'), 'body {}');
    fs.writeFileSync(path.join(appPath, 'script.js'), 'console.log(1)');

    const result = deleteWebApp(root, 1, 10, 'myapp');

    expect(result).toBe(true);
    expect(fs.existsSync(appPath)).toBe(false);

    fs.rmSync(root, { recursive: true, force: true });
  });

  it('returns false for non-existent app', () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), 'hermit-apps-'));
    const result = deleteWebApp(root, 1, 10, 'nonexistent');
    expect(result).toBe(false);
    fs.rmSync(root, { recursive: true, force: true });
  });
});
