import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';
import path from 'path';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');

  return {
    define: {
      __LOCAL_API__: JSON.stringify(env.VITE_LOCAL_API ?? ''),
    },
    plugins: [
      react(),
      VitePWA({
        strategies: 'injectManifest',
        srcDir: 'src',
        filename: 'sw.ts',
        registerType: 'prompt',
        injectManifest: {
          swDest: 'dist/sw.js',
        },
        manifest: {
          name: 'TPT Health',
          short_name: 'My Health',
          description: 'Your personal health portal — appointments, records, and queue check-in',
          theme_color: '#0d9488',
          background_color: '#ffffff',
          display: 'standalone',
          start_url: '/dashboard',
          scope: '/',
          icons: [
            { src: '/icons/icon-192.png', sizes: '192x192', type: 'image/png' },
            { src: '/icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any maskable' },
            { src: '/icons/apple-touch-icon.png', sizes: '180x180', type: 'image/png' },
          ],
        },
        devOptions: {
          enabled: true,
          type: 'module',
        },
      }),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        '@tpt/offline-store': path.resolve(__dirname, '../../packages/offline-store/src/index.ts'),
      },
    },
    server: {
      port: 3001,
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },
  };
});
