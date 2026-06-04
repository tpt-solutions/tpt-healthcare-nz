import { useEffect } from 'react';
import { encrypt, putEncrypted, setMeta, requestSync, checkAndWarnQuota } from '@tpt/offline-store';

/**
 * Offline sync for the staff app.
 *
 * On login (when a crypto key is available):
 *   - Fetches today's patient list and stores each record encrypted in IndexedDB.
 *   - Fetches today's appointments and queue entries.
 *   - Skips low-priority stores (encounters, prescriptions) when in power-save mode.
 *   - Checks storage quota after the batch write and warns if running low.
 *   - Registers Periodic Background Sync (Chrome/Edge) for 12-hourly background refresh.
 *
 * When connectivity is restored:
 *   - Registers a Background Sync tag so the SW flushes any queued mutations.
 */
export function useOfflineSync(cryptoKey: CryptoKey | null, isPowerSave = false) {
  useEffect(() => {
    if (!cryptoKey) return;
    prefetchTodaysData(cryptoKey, isPowerSave).catch((err) =>
      console.warn('[offline-sync] prefetch failed:', err)
    );
  }, [cryptoKey, isPowerSave]);

  // Re-register sync on reconnect
  useEffect(() => {
    const handleOnline = () => { requestSync().catch(() => {}); };
    window.addEventListener('online', handleOnline);
    return () => window.removeEventListener('online', handleOnline);
  }, []);

  // Register Periodic Background Sync (Chrome/Edge — silent no-op elsewhere)
  useEffect(() => {
    if (!cryptoKey) return;
    navigator.serviceWorker?.ready.then(async (reg) => {
      const ps = (reg as unknown as PeriodicSyncRegistration | undefined)?.periodicSync;
      if (!ps) return;
      try {
        await ps.register('tpt-data-refresh', { minInterval: 12 * 60 * 60 * 1000 });
      } catch { /* permission not granted or API not supported */ }
    });
  }, [cryptoKey]);
}

async function prefetchTodaysData(key: CryptoKey, isPowerSave: boolean) {
  // Fetch today's patient appointments
  const apptRes = await fetch('/api/v1/appointments?date=today&limit=200');
  if (apptRes.ok) {
    const appointments: Array<{ id: string }> = await apptRes.json();
    for (const appt of appointments) {
      const blob = await encrypt(key, JSON.stringify(appt));
      await putEncrypted('appointments', appt.id, blob);
    }
  }

  // Collect patient IDs from appointments and fetch their records
  const apptData: Array<{ patientId?: string }> = apptRes.ok
    ? await apptRes.clone().json().catch(() => [])
    : [];
  const patientIds = [...new Set(apptData.map((a) => a.patientId).filter(Boolean))] as string[];

  for (const pid of patientIds) {
    const patRes = await fetch(`/api/v1/patients/${pid}`);
    if (patRes.ok) {
      const patient: { id: string } = await patRes.json();
      const blob = await encrypt(key, JSON.stringify(patient));
      await putEncrypted('patients', patient.id, blob);
    }
  }

  // Fetch today's queue entries
  const queueRes = await fetch('/api/v1/queue?date=today');
  if (queueRes.ok) {
    const data: { entries?: Array<{ id: string }> } = await queueRes.json();
    for (const entry of data.entries ?? []) {
      const blob = await encrypt(key, JSON.stringify(entry));
      await putEncrypted('queue_entries', entry.id, blob);
    }
  }

  // Skip heavy historical data when on low battery or degraded network
  if (!isPowerSave) {
    const encRes = await fetch('/api/v1/encounters?recent=true&limit=50');
    if (encRes.ok) {
      const encounters: Array<{ id: string }> = await encRes.json();
      for (const enc of encounters) {
        const blob = await encrypt(key, JSON.stringify(enc));
        await putEncrypted('encounters', enc.id, blob);
      }
    }
  }

  await setMeta('lastSync', new Date().toISOString());
  await checkAndWarnQuota();
}

// Minimal type stubs for the Periodic Background Sync API (not yet in TS lib)
interface PeriodicSyncManager {
  register(tag: string, opts: { minInterval: number }): Promise<void>;
}
interface PeriodicSyncRegistration {
  periodicSync: PeriodicSyncManager;
}
