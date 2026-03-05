import { describe, expect, it } from 'vitest';
import { detectWebServer } from '../src/telegram';

describe('detectWebServer', () => {
  it('detects actual local web server output', () => {
    const output = 'Serving HTTP on 0.0.0.0 port 8080 (http://0.0.0.0:8080/) ...';
    expect(detectWebServer(output, 7)).toEqual({ url: '/preview/7/8080/', port: 8080 });
  });

  it('does not mistake node stack trace line numbers for web server ports', () => {
    const output = `node:internal/modules/cjs/loader:1143\nthrow err;\nError: Cannot find module '@libsql/client'\nRequire stack:\n- /app/workspace/agent.js`;
    expect(detectWebServer(output, 7)).toBeNull();
  });
});
