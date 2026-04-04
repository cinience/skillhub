import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: './static',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:10070',
      '/login': 'http://localhost:10070',
      '/logout': 'http://localhost:10070',
      '/.well-known': 'http://localhost:10070',
      '/healthz': 'http://localhost:10070',
    },
  },
})
