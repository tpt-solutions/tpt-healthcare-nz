import { useEffect } from 'react';

const PUSH_SUBSCRIBED_KEY = 'tpt:push:subscribed';

async function getVapidKey(): Promise<string> {
  const res = await fetch('/api/v1/push/vapid-key');
  if (!res.ok) throw new Error('Failed to fetch VAPID key');
  const { publicKey } = await res.json();
  return publicKey;
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  return Uint8Array.from([...rawData].map((c) => c.charCodeAt(0)));
}

export function usePushSetup() {
  useEffect(() => {
    if (localStorage.getItem(PUSH_SUBSCRIBED_KEY)) return;
    if (!('serviceWorker' in navigator) || !('PushManager' in window)) return;

    const setup = async () => {
      const permission = await Notification.requestPermission();
      if (permission !== 'granted') return;

      try {
        const vapidKey = await getVapidKey();
        const reg = await navigator.serviceWorker.ready;
        const sub = await reg.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: urlBase64ToUint8Array(vapidKey),
        });

        const { endpoint, keys } = sub.toJSON() as {
          endpoint: string;
          keys: { p256dh: string; auth: string };
        };

        const res = await fetch('/api/v1/push/subscribe', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ endpoint, keys }),
        });

        if (res.ok) {
          localStorage.setItem(PUSH_SUBSCRIBED_KEY, '1');
        }
      } catch (err) {
        // Non-fatal — push notifications are optional
        console.warn('Push setup failed:', err);
      }
    };

    setup();
  }, []);
}
