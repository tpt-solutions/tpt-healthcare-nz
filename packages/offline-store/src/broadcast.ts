/**
 * BroadcastChannel wrapper for multi-tab cache consistency.
 *
 * When a user has multiple tabs open (common for clinic staff), mutations enqueued
 * in one tab are broadcast to others so their in-memory state stays coherent without
 * requiring a full page reload or server round-trip.
 *
 * Works in both the main thread and service worker contexts.
 * Gracefully no-ops where BroadcastChannel is unavailable.
 */

const CHANNEL_NAME = 'tpt-sync';

export type BroadcastMsg =
  | { type: 'MUTATION_ENQUEUED'; item: { method: string; url: string; timestamp: number } }
  | { type: 'CACHE_UPDATED'; store: string; id: string }
  | { type: 'SYNC_COMPLETE' };

let _channel: BroadcastChannel | null = null;

function getChannel(): BroadcastChannel | null {
  if (typeof BroadcastChannel === 'undefined') return null;
  if (!_channel) _channel = new BroadcastChannel(CHANNEL_NAME);
  return _channel;
}

/** Notify other tabs that a mutation was added to the sync queue. */
export function broadcastMutation(item: { method: string; url: string; timestamp: number }): void {
  getChannel()?.postMessage({ type: 'MUTATION_ENQUEUED', item } satisfies BroadcastMsg);
}

/** Notify other tabs that a specific store record was written. */
export function broadcastCacheUpdate(store: string, id: string): void {
  getChannel()?.postMessage({ type: 'CACHE_UPDATED', store, id } satisfies BroadcastMsg);
}

/** Notify other tabs that the sync queue was successfully flushed. */
export function broadcastSyncComplete(): void {
  getChannel()?.postMessage({ type: 'SYNC_COMPLETE' } satisfies BroadcastMsg);
}

/**
 * Subscribe to peer messages from other tabs.
 * Returns an unsubscribe function — call it in your cleanup effect.
 */
export function listenForPeerUpdates(callback: (msg: BroadcastMsg) => void): () => void {
  const ch = getChannel();
  if (!ch) return () => {};
  const handler = (e: MessageEvent) => callback(e.data as BroadcastMsg);
  ch.addEventListener('message', handler);
  return () => ch.removeEventListener('message', handler);
}
