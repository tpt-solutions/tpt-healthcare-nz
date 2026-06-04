import { useState, useEffect, useRef } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { useSSEStream } from '../hooks/useSSEStream';

type CheckInState = 'idle' | 'checking-in' | 'waiting' | 'called' | 'done';

interface QueueEntry {
  entryId: string;
  position: number;
  estimatedWaitMinutes: number;
  room?: string;
}

const LOCATION_UPDATE_INTERVAL_MS = 10_000;
const LOCATION_MOVE_THRESHOLD_M = 20;

export function WaitingPage() {
  const { user } = useAuth();

  const [checkInState, setCheckInState] = useState<CheckInState>('idle');
  const [nhi, setNhi] = useState(user?.nhi ?? '');
  const [entry, setEntry] = useState<QueueEntry | null>(null);
  const [position, setPosition] = useState(0);
  const [estWait, setEstWait] = useState(0);
  const [roomHint, setRoomHint] = useState('');
  const [error, setError] = useState('');
  const [gpsEnabled, setGpsEnabled] = useState(false);
  const [locationShared, setLocationShared] = useState(false);

  const watchIdRef = useRef<number | null>(null);
  const lastLocRef = useRef<{ lat: number; lng: number } | null>(null);
  const locationTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // --- Check-in ---
  const handleCheckIn = async () => {
    const trimmed = nhi.trim().toUpperCase();
    if (!trimmed) {
      setError('Please enter your NHI number');
      return;
    }
    setError('');
    setCheckInState('checking-in');

    try {
      const res = await fetch(`/api/v1/queue/today/check-in`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ nhi: trimmed }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Check-in failed');
      }
      const data: QueueEntry = await res.json();
      setEntry(data);
      setPosition(data.position);
      setEstWait(data.estimatedWaitMinutes);
      setCheckInState('waiting');
      setSseEntryId(data.entryId);
      requestGpsPermission(data.entryId);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Check-in failed. Please try again.');
      setCheckInState('idle');
    }
  };

  // SSE URL — only set after successful check-in
  const [sseEntryId, setSseEntryId] = useState<string | null>(null);
  const sseUrl = sseEntryId ? `/api/v1/queue/today/entries/${sseEntryId}/stream` : null;

  const sseState = useSSEStream(sseUrl, {
    'position-updated': (data) => {
      const d = data as { position: number; estimatedWaitMinutes?: number };
      setPosition(d.position);
      setEstWait(d.estimatedWaitMinutes ?? 0);
    },
    'estimated-wait': (data) => {
      const d = data as { estimatedWaitMinutes: number };
      setEstWait(d.estimatedWaitMinutes);
    },
    'entry-called': (data) => {
      const d = data as { room?: string };
      setRoomHint(d.room ?? '');
      setCheckInState('called');
      stopGPS();
      navigator.serviceWorker?.ready.then((reg) => {
        reg.active?.postMessage({ type: 'QUEUE_CALLED', room: d.room });
      });
    },
  });

  // --- GPS ---
  const requestGpsPermission = (entryId: string) => {
    if (!('geolocation' in navigator)) return;
    // Don't auto-request; let user opt in
    setGpsEnabled(false);
    // Store entryId for later use in GPS toggle
    sessionStorage.setItem('tpt:queue:entryId', entryId);
  };

  const toggleGPS = () => {
    if (gpsEnabled) {
      stopGPS();
      setGpsEnabled(false);
    } else {
      startGPS();
    }
  };

  const startGPS = () => {
    if (!('geolocation' in navigator)) return;
    const entryId = entry?.entryId ?? sessionStorage.getItem('tpt:queue:entryId');
    if (!entryId) return;

    watchIdRef.current = navigator.geolocation.watchPosition(
      (pos) => {
        const { latitude: lat, longitude: lng, accuracy } = pos.coords;
        const last = lastLocRef.current;

        // Only send if we've moved more than the threshold (saves battery)
        if (last) {
          const dist = haversineM(last.lat, last.lng, lat, lng);
          if (dist < LOCATION_MOVE_THRESHOLD_M) return;
        }
        lastLocRef.current = { lat, lng };
        sendLocation(entryId, lat, lng, accuracy);
      },
      () => { setGpsEnabled(false); },
      { enableHighAccuracy: false, maximumAge: 15_000 }
    );
    setGpsEnabled(true);
    setLocationShared(true);

    // Heartbeat: send location every 10s even without movement
    locationTimerRef.current = setInterval(() => {
      const last = lastLocRef.current;
      if (!last) return;
      sendLocation(entryId, last.lat, last.lng, 0);
    }, LOCATION_UPDATE_INTERVAL_MS);
  };

  const stopGPS = () => {
    if (watchIdRef.current !== null) {
      navigator.geolocation.clearWatch(watchIdRef.current);
      watchIdRef.current = null;
    }
    if (locationTimerRef.current) {
      clearInterval(locationTimerRef.current);
      locationTimerRef.current = null;
    }
  };

  const sendLocation = async (entryId: string, lat: number, lng: number, accuracy: number) => {
    const queueId = 'today'; // resolved server-side from context
    await fetch(`/api/v1/queue/${queueId}/entries/${entryId}/location`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ lat, lng, accuracyMeters: accuracy }),
    }).catch(() => { /* non-fatal */ });
  };

  // Cleanup GPS on unmount (SSE cleanup is handled by useSSEStream)
  useEffect(() => {
    return () => { stopGPS(); };
  }, []);

  // --- Render states ---

  if (checkInState === 'called') {
    return (
      <div className="min-h-screen bg-teal-600 flex flex-col items-center justify-center p-6 text-white">
        <div className="text-6xl mb-6">🔔</div>
        <h1 className="text-3xl font-bold mb-3">Your turn!</h1>
        {roomHint && (
          <p className="text-xl mb-2">Please head to <strong>{roomHint}</strong></p>
        )}
        {!roomHint && (
          <p className="text-xl mb-2">Please come to reception</p>
        )}
        <p className="text-teal-200 text-sm mt-4">Thank you for your patience</p>
        <button
          onClick={() => setCheckInState('done')}
          className="mt-8 px-6 py-3 bg-white text-teal-700 font-semibold rounded-xl"
        >
          Done
        </button>
      </div>
    );
  }

  if (checkInState === 'waiting' && entry) {
    return (
      <div className="p-6 max-w-md mx-auto">
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-2xl font-bold text-gray-900">You're in the queue</h1>
          {sseState === 'reconnecting' && (
            <span className="inline-flex items-center gap-1.5 rounded-full bg-orange-100 px-3 py-1 text-xs font-medium text-orange-700">
              <svg className="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
              Reconnecting…
            </span>
          )}
          {sseState === 'offline' && (
            <span className="inline-flex items-center rounded-full bg-gray-100 px-3 py-1 text-xs font-medium text-gray-500">
              Offline
            </span>
          )}
        </div>

        {/* Position indicator */}
        <div className="bg-white rounded-2xl border border-gray-200 p-8 text-center mb-4">
          <p className="text-sm text-gray-500 mb-1">Your position</p>
          <p className="text-7xl font-black text-teal-600 leading-none">#{position}</p>
          {estWait > 0 && (
            <p className="text-gray-500 mt-3">
              Approx. <strong>{estWait}</strong> min wait
            </p>
          )}
        </div>

        {/* GPS toggle */}
        <div className="bg-white rounded-2xl border border-gray-200 p-5 mb-4">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium text-gray-900 text-sm">Share your location</p>
              <p className="text-xs text-gray-500 mt-0.5">
                {locationShared
                  ? 'Staff can see where you are'
                  : "You don't have to stay in the waiting room"}
              </p>
            </div>
            <button
              onClick={toggleGPS}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                gpsEnabled ? 'bg-teal-600' : 'bg-gray-200'
              }`}
              role="switch"
              aria-checked={gpsEnabled}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform ${
                  gpsEnabled ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>
          {!gpsEnabled && (
            <p className="text-xs text-gray-400 mt-2">
              Optional — turn on so staff can find you if you're waiting outside or in your car.
            </p>
          )}
        </div>

        <p className="text-xs text-center text-gray-400">
          We'll notify you when it's your turn. You can close this screen.
        </p>
      </div>
    );
  }

  // Idle / check-in form
  return (
    <div className="p-6 max-w-md mx-auto">
      <h1 className="text-2xl font-bold text-gray-900 mb-2">Check in</h1>
      <p className="text-gray-500 text-sm mb-6">
        Enter your NHI number to join the queue. You don't need to stay in the waiting room —
        we'll send you a notification when it's your turn.
      </p>

      <div className="bg-white rounded-2xl border border-gray-200 p-6">
        <label className="block text-sm font-medium text-gray-700 mb-1">NHI Number</label>
        <input
          type="text"
          value={nhi}
          onChange={(e) => setNhi(e.target.value.toUpperCase())}
          onKeyDown={(e) => e.key === 'Enter' && handleCheckIn()}
          placeholder="e.g. ZAB1234"
          maxLength={7}
          className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-gray-900 font-mono text-lg tracking-widest focus:outline-none focus:ring-2 focus:ring-teal-500 focus:border-transparent uppercase"
          autoComplete="off"
          spellCheck={false}
        />
        {error && (
          <p className="mt-2 text-sm text-red-600">{error}</p>
        )}

        <button
          onClick={handleCheckIn}
          disabled={checkInState === 'checking-in'}
          className="mt-4 w-full py-3 bg-teal-600 text-white font-semibold rounded-xl hover:bg-teal-700 disabled:opacity-60 transition-colors"
        >
          {checkInState === 'checking-in' ? 'Checking in…' : 'Check in'}
        </button>
      </div>

      <p className="text-xs text-center text-gray-400 mt-4">
        Your NHI is used only to find your appointment and will not be stored during your wait.
      </p>
    </div>
  );
}

// Haversine distance in metres between two lat/lng points.
function haversineM(lat1: number, lng1: number, lat2: number, lng2: number): number {
  const R = 6371000;
  const dLat = ((lat2 - lat1) * Math.PI) / 180;
  const dLng = ((lng2 - lng1) * Math.PI) / 180;
  const a =
    Math.sin(dLat / 2) ** 2 +
    Math.cos((lat1 * Math.PI) / 180) * Math.cos((lat2 * Math.PI) / 180) * Math.sin(dLng / 2) ** 2;
  return R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}
