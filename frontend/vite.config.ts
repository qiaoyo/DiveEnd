import path from 'path'
import {defineConfig} from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react({ fastRefresh: !process.env.VITEST })],
  define: {
    __DIVEEND_ROOT__: JSON.stringify(path.resolve(__dirname, '..')),
  },
  server: {
    fs: {
      allow: [path.resolve(__dirname, '..')],
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
  },
})
