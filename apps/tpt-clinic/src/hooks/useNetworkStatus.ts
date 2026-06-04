import { useEffect, useState, useCallback } from 'react';
import { getMeta } from '@tpt/offline-store';

export interface NetworkStatus {
  online: boolean;
  lastSynced: Date | null;
  isSyncing: boolean;
}

/**
 * Tracks network connectivity and last-sync timestamp.
 *
 * - `online`: mirrors navigator.onLine, updated via window online/offline events
 * - `lastSynced`: read from IndexedDB meta on mount, refreshed when the SW posts SYNC_COMPLETE
 * - `isSyncing`: true between a reconnect event and the next SYNC_COMPLETE message
 */
export function useNetworkStatus(): NetworkStatus {
  const [online, setOnline] = useState(() => navigator.onLine);
  const [lastSynced, setLastSynced] = useState<Date | null>(null);
  const [isSyncing, setIsSyncing] = useState(false);

  const refreshLastSynced = useCallback(async () => {
    try {
      const stored = await getMeta('lastSync');
      setLastSynced(stored ? new Date(stored) : null);
    } catch { /* IndexedDB may not be open yet */ }
  }, []);

  // Load lastSync on mount
  useEffect(() => {
    void refreshLastSynced();
  }, [refreshLastSynced]);

  useEffect(() => {
    const handleOnline = () => {
      setOnline(true);
      setIsSyncing(true);
    };
    const handleOffline = () => {
      setOnline(false);
      setIsSyncing(false);
    };

    window.addEventListener('online', handleOnline);
    window.addEventListener('offline', handleOffline);

    // Listen for SYNC_COMPLETE from the service worker
    const handleSwMessage = (event: MessageEvent) => {
      if (event.data?.type === 'SYNC_COMPLETE') {
        setIsSyncing(false);
        void refreshLastSynced();
      }
    };
    navigator.serviceWorker?.addEventListener('message', handleSwMessage);

    // Also listen via BroadcastChannel (same-device multi-tab)
    let bc: BroadcastChannel | null = null;
    if (typeof BroadcastChannel !== 'undefined') {
      bc = new BroadcastChannel('tpt-sync');
      bc.addEventListener('message', (e) => {
        if ((e.data as { type: string })?.type === 'SYNC_COMPLETE') {
          setIsSyncing(false);
          void refreshLastSynced();
        }
      });
    }

    return () => {
      window.removeEventListener('online', handleOnline);
      window.removeEventListener('offline', handleOffline);
      navigator.serviceWorker?.removeEventListener('message', handleSwMessage);
      bc?.close();
    };
  }, [refreshLastSynced]);

  return { online, lastSynced, isSyncing };
}

/** Format a Date as a human-readable relative time string (e.g. "3 min ago"). */
export function formatRelativeTime(date: Date): string {
  const secs = Math.round((Date.now() - date.getTime()) / 1000);
  if (secs < 60) return 'just now';
  const mins = Math.round(secs / 60);
  if (mins < 60) return `${mins} min ago`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return `${hours} hr ago`;
  return date.toLocaleDateString();
}
