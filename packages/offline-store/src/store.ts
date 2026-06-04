/**
 * IndexedDB schema for the tpt-healthcare offline cache.
 * All clinical object stores hold ciphertext — nothing is readable without the PIN-derived key.
 * The sync_queue stores plaintext mutation descriptors (patient IDs + action type, no PHI content).
 *
 * DB v2 adds the dead_letter store for mutations that have exhausted retries or hit permanent errors.
 */

import { openDB, type IDBPDatabase } from 'idb';
import type { EncryptedBlob } from './crypto.js';

const DB_NAME = 'tpt-offline-v1';
const DB_VERSION = 2;

export type OfflineStoreName =
  | 'patients'
  | 'appointments'
  | 'encounters'
  | 'prescriptions'
  | 'queue_entries'
  | 'sync_queue'
  | 'dead_letter'
  | 'meta';

export interface SyncQueueItem {
  id?: number;
  method: string;
  url: string;
  body: string; // JSON-serialised, no PHI
  timestamp: number;
  retryCount?: number;
  nextRetryAt?: number; // epoch ms; undefined / 0 means immediately due
}

export interface DeadLetterItem {
  id?: number;
  method: string;
  url: string;
  body: string;
  timestamp: number;
  failureReason: string;
  serverResponse?: string;
}

let _db: IDBPDatabase | null = null;

async function getDB(): Promise<IDBPDatabase> {
  if (_db) return _db;
  _db = await openDB(DB_NAME, DB_VERSION, {
    upgrade(db, oldVersion) {
      // v1 stores — also created on fresh installs (oldVersion === 0)
      if (oldVersion < 1) {
        for (const name of ['patients', 'appointments', 'encounters', 'prescriptions', 'queue_entries'] as const) {
          if (!db.objectStoreNames.contains(name)) {
            db.createObjectStore(name); // keyed by patient/resource ID
          }
        }
        if (!db.objectStoreNames.contains('sync_queue')) {
          db.createObjectStore('sync_queue', { keyPath: 'id', autoIncrement: true });
        }
        if (!db.objectStoreNames.contains('meta')) {
          db.createObjectStore('meta'); // key-value
        }
      }
      // v2: dead-letter store for permanently failed mutations
      if (oldVersion < 2) {
        if (!db.objectStoreNames.contains('dead_letter')) {
          db.createObjectStore('dead_letter', { keyPath: 'id', autoIncrement: true });
        }
      }
    },
  });
  return _db;
}

/** Store an encrypted blob under the given key in the named store. */
export async function putEncrypted(store: OfflineStoreName, key: string, blob: EncryptedBlob): Promise<void> {
  const db = await getDB();
  const serialised = {
    iv: btoa(String.fromCharCode(...blob.iv)),
    ciphertext: blob.ciphertext,
  };
  await db.put(store as string, serialised, key);
}

/** Retrieve and deserialise an encrypted blob. Returns null if not found. */
export async function getEncrypted(store: OfflineStoreName, key: string): Promise<EncryptedBlob | null> {
  const db = await getDB();
  const raw = await db.get(store as string, key);
  if (!raw) return null;
  return {
    iv: Uint8Array.from(atob(raw.iv as string), (c) => c.charCodeAt(0)),
    ciphertext: raw.ciphertext as ArrayBuffer,
  };
}

/** List all keys in a store. */
export async function getAllKeys(store: OfflineStoreName): Promise<string[]> {
  const db = await getDB();
  return (await db.getAllKeys(store as string)) as string[];
}

/** List all encrypted blobs in a store. */
export async function getAll(store: OfflineStoreName): Promise<Array<{ key: string; blob: EncryptedBlob }>> {
  const db = await getDB();
  const cursor = await db.transaction(store as string).store.openCursor();
  const results: Array<{ key: string; blob: EncryptedBlob }> = [];
  let c = cursor;
  while (c) {
    const raw = c.value as { iv: string; ciphertext: ArrayBuffer };
    results.push({
      key: c.key as string,
      blob: {
        iv: Uint8Array.from(atob(raw.iv), (ch) => ch.charCodeAt(0)),
        ciphertext: raw.ciphertext,
      },
    });
    c = await c.continue();
  }
  return results;
}

/** Add a pending mutation to the sync queue. */
export async function enqueueMutation(item: Omit<SyncQueueItem, 'id'>): Promise<void> {
  const db = await getDB();
  await db.add('sync_queue', item);
  // Notify other tabs that a mutation was enqueued
  const { broadcastMutation } = await import('./broadcast.js');
  broadcastMutation({ method: item.method, url: item.url, timestamp: item.timestamp });
}

/** Return only items whose nextRetryAt is unset or in the past (ready to process). */
export async function getDueSyncItems(): Promise<SyncQueueItem[]> {
  const db = await getDB();
  const all = (await db.getAll('sync_queue')) as SyncQueueItem[];
  const now = Date.now();
  return all.filter((item) => !item.nextRetryAt || item.nextRetryAt <= now);
}

/** Update an existing sync queue item in-place (used to persist retry backoff). */
export async function updateSyncItem(item: SyncQueueItem): Promise<void> {
  const db = await getDB();
  await db.put('sync_queue', item);
}

/** Delete a single sync queue item by its autoincrement id. */
export async function deleteSyncItem(id: number): Promise<void> {
  const db = await getDB();
  await db.delete('sync_queue', id);
}

/** Retrieve and clear the entire sync queue atomically (used on logout/full wipe). */
export async function drainSyncQueue(): Promise<SyncQueueItem[]> {
  const db = await getDB();
  const tx = db.transaction('sync_queue', 'readwrite');
  const items = (await tx.store.getAll()) as SyncQueueItem[];
  await tx.store.clear();
  await tx.done;
  return items;
}

// ---------------------------------------------------------------------------
// Dead-letter store
// ---------------------------------------------------------------------------

/** Move a failed sync item to the dead-letter store and remove it from the queue. */
export async function addToDeadLetter(
  item: SyncQueueItem,
  failureReason: string,
  serverResponse?: string,
): Promise<void> {
  const db = await getDB();
  const dlItem: Omit<DeadLetterItem, 'id'> = {
    method: item.method,
    url: item.url,
    body: item.body,
    timestamp: item.timestamp,
    failureReason,
    serverResponse,
  };
  await db.add('dead_letter', dlItem);
  if (item.id !== undefined) {
    await db.delete('sync_queue', item.id);
  }
}

/** Return all dead-letter items (failed mutations awaiting user review). */
export async function getDeadLetterItems(): Promise<DeadLetterItem[]> {
  const db = await getDB();
  return db.getAll('dead_letter') as Promise<DeadLetterItem[]>;
}

/** Discard a dead-letter item after the user has acknowledged or resolved it. */
export async function clearDeadLetterItem(id: number): Promise<void> {
  const db = await getDB();
  await db.delete('dead_letter', id);
}

// ---------------------------------------------------------------------------
// Meta store
// ---------------------------------------------------------------------------

/** Read a meta value. */
export async function getMeta(key: string): Promise<string | undefined> {
  const db = await getDB();
  return db.get('meta', key) as Promise<string | undefined>;
}

/** Write a meta value. */
export async function setMeta(key: string, value: string): Promise<void> {
  const db = await getDB();
  await db.put('meta', value, key);
}

/**
 * Delete all clinical caches, the sync queue, and dead-letter items.
 * Called on PIN lockout (5 wrong attempts) or explicit logout+wipe.
 * HISO 10064.1 §4.5 — brute-force protection via data destruction.
 */
export async function clearAll(): Promise<void> {
  const db = await getDB();
  const tx = db.transaction(
    ['patients', 'appointments', 'encounters', 'prescriptions', 'queue_entries', 'sync_queue', 'dead_letter', 'meta'],
    'readwrite'
  );
  await Promise.all([
    tx.objectStore('patients').clear(),
    tx.objectStore('appointments').clear(),
    tx.objectStore('encounters').clear(),
    tx.objectStore('prescriptions').clear(),
    tx.objectStore('queue_entries').clear(),
    tx.objectStore('sync_queue').clear(),
    tx.objectStore('dead_letter').clear(),
    tx.objectStore('meta').clear(),
  ]);
  await tx.done;
  _db = null; // force re-open on next access
}
