import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import qiankun from 'vite-plugin-qiankun';
import path from 'path';

export default defineConfig({
  plugins: [
    react(),
    qiankun('app-dashboard', { useDevMode: true }),
  ],
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  server: {
    port: 3001,
    cors: true,
    headers: { 'Access-Control-Allow-Origin': '*' },
  },
});
