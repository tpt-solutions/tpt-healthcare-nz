/**
 * Background Sync helpers with exponential backoff and dead-letter handling.
 *
 * On network error: retries with exponential backoff (30s → 60s → 120s … capped at 5 min).
 * After 5 failures, or on HTTP 4xx/409: moves the item to the dead_letter store.
 *
 * The service worker fires a 'sync' event with tag 'tpt-pending-writes' when connectivity
 * is restored. This module provides the flush logic for both the SW and the main thread.
 */

import {
  getDueSyncItems,
  deleteSyncItem,
  updateSyncItem,
  addToDeadLetter,
  type SyncQueueItem,
} from './store.js';

export const SYNC_TAG = 'tpt-pending-writes';

const MAX_RETRIES = 5;
const BASE_BACKOFF_MS = 30_000; // doubles each retry, capped at MAX_BACKOFF_MS
const MAX_BACKOFF_MS = 300_000; // 5 minutes

/**
 * Register a Background Sync so the SW flushes the queue once online.
 * Falls back to immediate flush if Background Sync API is unavailable (e.g. Firefox).
 */
export async function requestSync(): Promise<void> {
  if (!('serviceWorker' in navigator)) return;
  const reg = await navigator.serviceWorker.ready;
  if ('sync' in reg) {
    await (reg as ServiceWorkerRegistration & { sync: { register(tag: string): Promise<void> } }).sync.register(
      SYNC_TAG
    );
  } else {
    await flushSyncQueue();
  }
}

/**
 * Replay all due pending mutations against the server.
 * Called by the service worker's 'sync' handler, or directly from the page as a fallback.
 * Items not yet due (backoff window not elapsed) are skipped.
 */
export async function flushSyncQueue(): Promise<void> {
  const items = await getDueSyncItems();
  if (items.length === 0) return;

  for (const item of items) {
    await replayMutation(item);
  }
}

async function replayMutation(item: SyncQueueItem): Promise<void> {
  try {
    const res = await fetch(item.url, {
      method: item.method,
      headers: { 'Content-Type': 'application/json' },
      body: item.method !== 'GET' && item.method !== 'DELETE' ? item.body : undefined,
    });

    if (res.ok) {
      // Success — remove from queue
      if (item.id !== undefined) await deleteSyncItem(item.id);
      return;
    }

    // 409 Conflict: server rejected due to concurrent change — dead letter with server body
    if (res.status === 409) {
      const serverResponse = await res.text().catch(() => '');
      await addToDeadLetter(item, `HTTP 409 Conflict`, serverResponse);
      return;
    }

    // Other 4xx: permanent client error — dead letter immediately
    if (res.status >= 400 && res.status < 500) {
      await addToDeadLetter(item, `HTTP ${res.status} — permanent failure`);
      return;
    }

    // 5xx / unexpected: treat as transient, fall into catch via thrown error
    throw new Error(`HTTP ${res.status}`);

  } catch (err) {
    // Network error or 5xx thrown above — schedule retry with backoff
    const retryCount = (item.retryCount ?? 0) + 1;

    if (retryCount > MAX_RETRIES) {
      const reason = err instanceof Error ? err.message : String(err);
      await addToDeadLetter(item, `Max retries exceeded (${MAX_RETRIES}): ${reason}`);
      return;
    }

    const backoffMs = Math.min(BASE_BACKOFF_MS * Math.pow(2, retryCount - 1), MAX_BACKOFF_MS);
    const updated: SyncQueueItem = {
      ...item,
      retryCount,
      nextRetryAt: Date.now() + backoffMs,
    };
    await updateSyncItem(updated);
    console.warn(
      `[sync] retry ${retryCount}/${MAX_RETRIES} scheduled for ${item.method} ${item.url} in ${Math.round(backoffMs / 1000)}s`
    );
  }
}
