import { defineConfig } from '@playwright/test';
import { tmpdir } from 'node:os';
export default defineConfig({
  testDir: './e2e', testMatch: '**/*.e2e.ts', workers: 1, timeout: 30000,
  outputDir: process.env.UI_ARTIFACT_DIR || `${tmpdir()}/glipz-ui-tests`,
  reporter: 'list',
  globalSetup: './e2e/setup.ts', globalTeardown: './e2e/cleanup.ts',
  use: { baseURL: 'http://127.0.0.1:8080', locale: 'ja-JP', viewport: { width:1440,height:900 },
    launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE }, screenshot: 'only-on-failure', trace: 'retain-on-failure' },
});
