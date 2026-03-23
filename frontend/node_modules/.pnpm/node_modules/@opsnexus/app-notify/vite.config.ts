import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import qiankun from 'vite-plugin-qiankun';
import path from 'path';

export default defineConfig({
  plugins: [
    react(),
    qiankun('app-notify', { useDevMode: true }),
  ],
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  server: {
    port: 3007,
    cors: true,
    headers: { 'Access-Control-Allow-Origin': '*' },
  },
});
