/// <reference lib="webworker" />
import { precacheAndRoute, cleanupOutdatedCaches } from 'workbox-precaching';
import { CacheFirst } from 'workbox-strategies';
import { registerRoute } from 'workbox-routing';
import { ExpirationPlugin } from 'workbox-expiration';
import { SYNC_TAG } from '@tpt/offline-store';

declare const self: ServiceWorkerGlobalScope;
// Compile-time fallback: VITE_LOCAL_API env var (empty string if not set).
// Runtime discovery via discoverLocalAPI() supplements or replaces this.
declare const __LOCAL_API__: string;

cleanupOutdatedCaches();
precacheAndRoute(self.__WB_MANIFEST);

// Track power-save mode — reduces non-critical background work when battery is low
// or the network connection is degraded.
let powerSaveMode = false;

// Resolved local API URL (compile-time or runtime-discovered).
let discoveredLocalAPI: string | null = __LOCAL_API__ || null;

// --- Activate: probe for a local API server on the LAN ---
self.addEventListener('activate', (event) => {
  event.waitUntil(
    discoverLocalAPI().then((url) => {
      if (url) discoveredLocalAPI = url;
    })
  );
});

/**
 * Probe well-known local addresses for the interop server.
 * Results are persisted to IndexedDB meta so future activations skip probing.
 */
async function discoverLocalAPI(): Promise<string | null> {
  // Skip if already configured at build time
  if (__LOCAL_API__) return null;

  const candidates: string[] = [];

  // Check previously discovered URL first
  try {
    const { getMeta } = await import('@tpt/offline-store');
    const stored = await getMeta('localAPI');
    if (stored) candidates.push(stored);
  } catch { /* ignore */ }

  candidates.push('http://localhost:8080', 'http://tpt-interop.local:8080');

  for (const url of candidates) {
    try {
      const ctrl = new AbortController();
      const timeoutId = setTimeout(() => ctrl.abort(), 1000);
      const res = await fetch(`${url}/healthz`, { signal: ctrl.signal });
      clearTimeout(timeoutId);
      if (res.ok) {
        try {
          const { setMeta } = await import('@tpt/offline-store');
          await setMeta('localAPI', url);
        } catch { /* ignore */ }
        return url;
      }
    } catch { /* try next candidate */ }
  }
  return null;
}

// --- API route: primary → local LAN → cache → offline response ---
registerRoute(
  ({ url }) => url.pathname.startsWith('/api/'),
  async ({ request }) => {
    // 1. Try the primary origin
    try {
      const res = await fetch(request.clone());
      const cache = await caches.open('clinical-api-cache');
      cache.put(request, res.clone());
      return res;
    } catch {
      // 2. Try discovered local LAN server (UPS-backed on-premise machine)
      const localBase = discoveredLocalAPI;
      if (localBase) {
        try {
          const localUrl = request.url.replace(self.location.origin, localBase);
          const res = await fetch(new Request(localUrl, request));
          return res;
        } catch { /* fall through */ }
      }

      // 3. Serve from cache (offline mode — today's patients were pre-fetched)
      const cached = await caches.match(request);
      if (cached) return cached;

      // 4. Structured offline response
      return new Response(
        JSON.stringify({ error: 'offline', message: 'No network or cache available' }),
        { status: 503, headers: { 'Content-Type': 'application/json' } }
      );
    }
  }
);

// Static assets: cache-first (fonts, images, icons)
registerRoute(
  ({ request }) => request.destination === 'font' || request.destination === 'image',
  new CacheFirst({
    cacheName: 'static-assets',
    plugins: [
      new ExpirationPlugin({ maxEntries: 60, maxAgeSeconds: 60 * 60 * 24 * 30 }),
    ],
  })
);

// --- Background Sync: flush queued mutations when back online ---
self.addEventListener('sync', (event) => {
  if (event.tag === SYNC_TAG) {
    event.waitUntil(flushAndNotify());
  }
});

// --- Periodic Background Sync: refresh cached data every 12 hours (Chrome/Edge) ---
self.addEventListener('periodicsync', (event) => {
  if ((event as { tag: string }).tag === 'tpt-data-refresh') {
    (event as ExtendableEvent).waitUntil(flushAndNotify());
  }
});

async function flushAndNotify(): Promise<void> {
  const { flushSyncQueue, broadcastSyncComplete } = await import('@tpt/offline-store');
  await flushSyncQueue();
  // Notify all open tabs that sync completed (updates lastSynced in useNetworkStatus)
  const clients = await self.clients.matchAll({ includeUncontrolled: true });
  for (const client of clients) {
    client.postMessage({ type: 'SYNC_COMPLETE' });
  }
  broadcastSyncComplete();
}

// --- Messages from the page ---
self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
  if (event.data?.type === 'POWER_SAVE_ON') {
    powerSaveMode = true;
    // Throttle SSE on all clients: the page's useSSEStream respects this
    notifyClients({ type: 'THROTTLE_SSE', enabled: true });
  }
  if (event.data?.type === 'POWER_SAVE_OFF') {
    powerSaveMode = false;
    notifyClients({ type: 'THROTTLE_SSE', enabled: false });
  }
});

function notifyClients(msg: unknown): void {
  self.clients.matchAll({ includeUncontrolled: true }).then((clients) => {
    for (const client of clients) client.postMessage(msg);
  });
}

// Keep TS happy — powerSaveMode gates prefetch frequency in notifyClients path
void powerSaveMode;
