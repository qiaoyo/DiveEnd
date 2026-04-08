import path from 'path'
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  define: {
    __DIVEEND_ROOT__: JSON.stringify(path.resolve(__dirname, '..')),
  },
  server: {
    fs: {
      allow: [path.resolve(__dirname, '..')],
    },
  },
})
