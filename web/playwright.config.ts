import {defineConfig, devices} from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  use: {baseURL: 'http://127.0.0.1:8080'},
  projects: [
    {name: 'chromium', use: {...devices['Desktop Chrome']}},
    {name: 'mobile', use: {...devices['Pixel 7']}},
  ],
})
