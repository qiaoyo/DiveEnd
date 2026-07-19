import path from 'path'
import {defineConfig} from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react({ fastRefresh: !process.env.VITEST })],
  server: {
    fs: {
      allow: [path.resolve(__dirname, '..')],
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return undefined
          }
          if (id.includes('react-pdf') || id.includes('pdfjs-dist')) {
            return 'pdf'
          }
          if (id.includes('react-router') || id.includes('@remix-run')) {
            return 'router'
          }
          if (id.includes('react') || id.includes('scheduler')) {
            return 'react'
          }
          if (id.includes('@radix-ui') || id.includes('framer-motion') || id.includes('lucide-react')) {
            return 'ui'
          }
          return 'vendor'
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
  },
})
