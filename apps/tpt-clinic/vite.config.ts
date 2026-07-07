import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { VitePWA } from 'vite-plugin-pwa';
import path from 'path';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');

  return {
    define: {
      // Injected into the service worker so it can fall back to a local LAN server
      // when the primary internet connection is unavailable.
      // Set VITE_LOCAL_API=https://10.0.0.1:8080 in .env.local for on-premise deployments.
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
          name: 'TPT Clinic',
          short_name: 'Clinic',
          description: 'Clinical staff portal — patients, queue management, and consultations',
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
      port: 3000,
      proxy: {
        // tpt-doctor (the GP clinical management service) owns /api/v1/patients,
        // /appointments, /prescriptions, etc. Override with VITE_API_PROXY_TARGET
        // to point at a different backend (e.g. interop) for local development.
        '/api': {
          target: env.VITE_API_PROXY_TARGET || 'http://localhost:8082',
          changeOrigin: true,
        },
      },
    },
  };
});
