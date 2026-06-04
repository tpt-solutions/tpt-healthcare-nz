/**
 * Storage quota monitoring.
 * Warns when IndexedDB + Cache API usage exceeds 80% of the browser's allocated quota,
 * and emits a StorageWarningEvent on window so the UI can surface a banner.
 */

export interface StorageQuota {
  usedMB: number;
  totalMB: number;
  pct: number;
}

export async function checkStorageQuota(): Promise<StorageQuota> {
  if (typeof navigator === 'undefined' || !('storage' in navigator) || !('estimate' in navigator.storage)) {
    return { usedMB: 0, totalMB: 0, pct: 0 };
  }
  const { usage = 0, quota = 0 } = await navigator.storage.estimate();
  const usedMB = usage / (1024 * 1024);
  const totalMB = quota / (1024 * 1024);
  const pct = quota > 0 ? (usage / quota) * 100 : 0;
  return { usedMB, totalMB, pct };
}

/**
 * Check quota and dispatch a StorageWarningEvent on window if usage exceeds thresholds.
 * Call this after large batch writes (e.g. prefetch) to give the UI a chance to react.
 */
export async function checkAndWarnQuota(): Promise<void> {
  const q = await checkStorageQuota();
  if (q.pct <= 80 || typeof window === 'undefined') return;

  const level = q.pct > 95 ? 'critical' : 'warning';
  window.dispatchEvent(
    new CustomEvent<StorageQuota & { level: 'warning' | 'critical' }>('tpt:storage-warning', {
      detail: { ...q, level },
    })
  );

  if (level === 'critical') {
    await evictOldRecords();
  }
}

/** Evict oldest 20% of low-priority stores (encounters, prescriptions) by key order. */
async function evictOldRecords(): Promise<void> {
  try {
    const { openDB } = await import('idb');
    const db = await openDB('tpt-offline-v1');
    for (const storeName of ['encounters', 'prescriptions'] as const) {
      if (!db.objectStoreNames.contains(storeName)) continue;
      const keys = await db.getAllKeys(storeName);
      const evictCount = Math.ceil(keys.length * 0.2);
      for (let i = 0; i < evictCount; i++) {
        await db.delete(storeName, keys[i]);
      }
    }
    console.warn(`[quota] evicted old records from encounters/prescriptions due to storage pressure`);
  } catch {
    // Non-fatal — ignore eviction errors
  }
}
