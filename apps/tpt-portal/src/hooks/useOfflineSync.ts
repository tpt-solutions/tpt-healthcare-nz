import { useEffect } from 'react';
import { encrypt, putEncrypted, setMeta, requestSync, checkAndWarnQuota } from '@tpt/offline-store';

/**
 * Offline sync for the patient portal.
 * Caches the logged-in patient's own record, their appointments, and prescriptions.
 * Skips prescriptions in power-save mode to conserve bandwidth.
 * Registers Periodic Background Sync (Chrome/Edge) for 12-hourly background refresh.
 */
export function useOfflineSync(cryptoKey: CryptoKey | null, isPowerSave = false) {
  useEffect(() => {
    if (!cryptoKey) return;
    prefetchPatientData(cryptoKey, isPowerSave).catch((err) =>
      console.warn('[offline-sync] prefetch failed:', err)
    );
  }, [cryptoKey, isPowerSave]);

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

async function prefetchPatientData(key: CryptoKey, isPowerSave: boolean) {
  // Own patient record (always fetch)
  const meRes = await fetch('/api/v1/patients/me');
  if (meRes.ok) {
    const me: { id: string } = await meRes.json();
    const blob = await encrypt(key, JSON.stringify(me));
    await putEncrypted('patients', me.id, blob);
  }

  // Upcoming appointments (always fetch)
  const apptRes = await fetch('/api/v1/appointments?upcoming=true&limit=10');
  if (apptRes.ok) {
    const appointments: Array<{ id: string }> = await apptRes.json();
    for (const appt of appointments) {
      const blob = await encrypt(key, JSON.stringify(appt));
      await putEncrypted('appointments', appt.id, blob);
    }
  }

  // Active prescriptions — skip on degraded connectivity to save bandwidth
  if (!isPowerSave) {
    const rxRes = await fetch('/api/v1/prescriptions?status=active');
    if (rxRes.ok) {
      const rxList: Array<{ id: string }> = await rxRes.json();
      for (const rx of rxList) {
        const blob = await encrypt(key, JSON.stringify(rx));
        await putEncrypted('prescriptions', rx.id, blob);
      }
    }
  }

  await setMeta('lastSync', new Date().toISOString());
  await checkAndWarnQuota();
}

interface PeriodicSyncManager {
  register(tag: string, opts: { minInterval: number }): Promise<void>;
}
interface PeriodicSyncRegistration {
  periodicSync: PeriodicSyncManager;
}
