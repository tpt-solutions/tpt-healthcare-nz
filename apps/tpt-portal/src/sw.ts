/// <reference lib="webworker" />
import { precacheAndRoute, cleanupOutdatedCaches } from 'workbox-precaching';
import { CacheFirst } from 'workbox-strategies';
import { registerRoute } from 'workbox-routing';
import { ExpirationPlugin } from 'workbox-expiration';
import { SYNC_TAG } from '@tpt/offline-store';

declare const self: ServiceWorkerGlobalScope;
declare const __LOCAL_API__: string;

cleanupOutdatedCaches();
precacheAndRoute(self.__WB_MANIFEST);

let powerSaveMode = false;
let discoveredLocalAPI: string | null = __LOCAL_API__ || null;

// --- Activate: probe for local API server ---
self.addEventListener('activate', (event) => {
  event.waitUntil(
    discoverLocalAPI().then((url) => {
      if (url) discoveredLocalAPI = url;
    })
  );
});

async function discoverLocalAPI(): Promise<string | null> {
  if (__LOCAL_API__) return null;

  const candidates: string[] = [];
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
    try {
      const res = await fetch(request.clone());
      const cache = await caches.open('api-cache');
      cache.put(request, res.clone());
      return res;
    } catch {
      const localBase = discoveredLocalAPI;
      if (localBase) {
        try {
          const localUrl = request.url.replace(self.location.origin, localBase);
          return await fetch(new Request(localUrl, request));
        } catch { /* fall through */ }
      }
      const cached = await caches.match(request);
      if (cached) return cached;
      return new Response(
        JSON.stringify({ error: 'offline', message: 'No network or cache available' }),
        { status: 503, headers: { 'Content-Type': 'application/json' } }
      );
    }
  }
);

// Static assets: cache-first
registerRoute(
  ({ request }) => request.destination === 'font' || request.destination === 'image',
  new CacheFirst({
    cacheName: 'static-assets',
    plugins: [
      new ExpirationPlugin({ maxEntries: 60, maxAgeSeconds: 60 * 60 * 24 * 30 }),
    ],
  })
);

// --- Background Sync ---
self.addEventListener('sync', (event) => {
  if (event.tag === SYNC_TAG) {
    event.waitUntil(flushAndNotify());
  }
});

// --- Periodic Background Sync (Chrome/Edge) ---
self.addEventListener('periodicsync', (event) => {
  if ((event as { tag: string }).tag === 'tpt-data-refresh') {
    (event as ExtendableEvent).waitUntil(flushAndNotify());
  }
});

async function flushAndNotify(): Promise<void> {
  const { flushSyncQueue, broadcastSyncComplete } = await import('@tpt/offline-store');
  await flushSyncQueue();
  const clients = await self.clients.matchAll({ includeUncontrolled: true });
  for (const client of clients) {
    client.postMessage({ type: 'SYNC_COMPLETE' });
  }
  broadcastSyncComplete();
}

// --- VAPID push notification (appointment reminders + queue called) ---
self.addEventListener('push', (event) => {
  const data = event.data?.json() as {
    title: string;
    body: string;
    tag?: string;
    url?: string;
    requireInteraction?: boolean;
  };
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      icon: '/icons/icon-192.png',
      badge: '/icons/badge-72.png',
      tag: data.tag ?? 'tpt-notification',
      data: { url: data.url ?? '/' },
      requireInteraction: data.requireInteraction ?? false,
    })
  );
});

// --- Messages from the page ---
self.addEventListener('message', (event) => {
  if (event.data?.type === 'QUEUE_CALLED') {
    self.registration.showNotification('Your turn!', {
      body: event.data.room ? `Please head to ${event.data.room as string}` : 'Please come to reception',
      icon: '/icons/icon-192.png',
      badge: '/icons/badge-72.png',
      tag: 'queue-called',
      requireInteraction: true,
      data: { url: '/waiting' },
    });
  }
  if (event.data?.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
  if (event.data?.type === 'POWER_SAVE_ON') {
    powerSaveMode = true;
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

// Notification click → navigate to relevant page
self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url: string = (event.notification.data as { url?: string })?.url ?? '/';
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      for (const client of clientList) {
        if (client.url.includes(self.location.origin) && 'focus' in client) {
          client.navigate(url);
          return client.focus();
        }
      }
      return self.clients.openWindow(url);
    })
  );
});

void powerSaveMode;
