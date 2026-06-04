import { useEffect, useRef, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useSSEStream } from '@/hooks/useSSEStream';
import 'leaflet/dist/leaflet.css';
import L from 'leaflet';

interface QueueEntry {
  id: string;
  patientInitials: string;
  position: number;
  status: 'waiting' | 'called' | 'in_progress' | 'done' | 'skipped' | 'left';
  checkedInAt: string;
  calledAt?: string;
  waitMinutes?: number;
  roomHint?: string;
  location?: { lat: number; lng: number };
}

interface QueueStats {
  waitingCount: number;
  avgWaitMinutes: number;
}

// Auckland CBD centre as default (overridden by clinic tenant coordinates in prod)
const DEFAULT_LAT = -36.8485;
const DEFAULT_LNG = 174.7633;

const STATUS_BADGE: Record<QueueEntry['status'], { label: string; cls: string }> = {
  waiting:     { label: 'Waiting',     cls: 'bg-amber-100 text-amber-700' },
  called:      { label: 'Called',      cls: 'bg-blue-100 text-blue-700' },
  in_progress: { label: 'With clinician', cls: 'bg-purple-100 text-purple-700' },
  done:        { label: 'Done',        cls: 'bg-green-100 text-green-700' },
  skipped:     { label: 'Skipped',     cls: 'bg-gray-100 text-gray-500' },
  left:        { label: 'Left',        cls: 'bg-gray-100 text-gray-400' },
};

function waitLabel(checkedInAt: string): string {
  const mins = Math.round((Date.now() - new Date(checkedInAt).getTime()) / 60000);
  if (mins < 1) return '< 1 min';
  return `${mins} min`;
}

export default function QueuePage() {
  const [queueID, setQueueID] = useState<string | null>(null);
  const [entries, setEntries] = useState<QueueEntry[]>([]);
  const [stats, setStats] = useState<QueueStats>({ waitingCount: 0, avgWaitMinutes: 0 });
  const [calling, setCalling] = useState(false);
  const [selectedEntry, setSelectedEntry] = useState<string | null>(null);
  const [showMap, setShowMap] = useState(false);
  const [loadError, setLoadError] = useState('');

  const mapRef = useRef<HTMLDivElement>(null);
  const leafletMap = useRef<L.Map | null>(null);
  const markersRef = useRef<Map<string, L.Marker>>(new Map());

  // --- Queue bootstrap ---
  useEffect(() => {
    fetch('/api/v1/queue', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'General Practice' }),
    })
      .then(r => r.json())
      .then((q: { id: string }) => {
        setQueueID(q.id);
        return fetch(`/api/v1/queue/${q.id}`);
      })
      .then(r => r.json())
      .then((data: { entries: QueueEntry[] }) => {
        setEntries(data.entries ?? []);
      })
      .catch(() => setLoadError('Could not load the queue. Please refresh.'));
  }, []);

  // --- SSE with automatic backoff reconnection ---
  const sseUrl = queueID ? `/api/v1/queue/${queueID}/stream` : null;
  const sseState = useSSEStream(sseUrl, {
    'entry-checked-in': (data) => {
      const entry = data as QueueEntry;
      setEntries(prev => {
        const existing = prev.findIndex(x => x.id === entry.id);
        if (existing >= 0) return prev.map((x, i) => i === existing ? entry : x);
        return [...prev, entry].sort((a, b) => a.position - b.position);
      });
    },
    'entry-called': (data) => {
      const { entryId, room } = data as { entryId: string; room?: string };
      setEntries(prev => prev.map(x =>
        x.id === entryId ? { ...x, status: 'called', roomHint: room, calledAt: new Date().toISOString() } : x
      ));
    },
    'entry-updated': (data) => {
      const { entryId, status } = data as { entryId: string; status: QueueEntry['status'] };
      setEntries(prev => prev.map(x => x.id === entryId ? { ...x, status } : x));
    },
    'entry-location-updated': (data) => {
      const { entryId, lat, lng } = data as { entryId: string; lat: number; lng: number };
      setEntries(prev => prev.map(x =>
        x.id === entryId ? { ...x, location: { lat, lng } } : x
      ));
      updateMapPin(entryId, lat, lng);
    },
    'queue-stats': (data) => {
      setStats(data as QueueStats);
    },
  });

  // --- Leaflet map ---
  useEffect(() => {
    if (!showMap || !mapRef.current || leafletMap.current) return;

    leafletMap.current = L.map(mapRef.current).setView([DEFAULT_LAT, DEFAULT_LNG], 16);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© OpenStreetMap contributors',
    }).addTo(leafletMap.current);

    // Add clinic pin
    L.marker([DEFAULT_LAT, DEFAULT_LNG], {
      icon: L.divIcon({
        className: '',
        html: '<div style="background:#0d9488;color:#fff;border-radius:50%;width:32px;height:32px;display:flex;align-items:center;justify-content:center;font-size:14px;box-shadow:0 2px 4px rgba(0,0,0,.3)">🏥</div>',
        iconSize: [32, 32],
        iconAnchor: [16, 16],
      }),
    }).addTo(leafletMap.current).bindTooltip('Clinic');

    // Add existing location pins
    entries.forEach(e => {
      if (e.location) updateMapPin(e.id, e.location.lat, e.location.lng);
    });
  }, [showMap]);

  const updateMapPin = (entryId: string, lat: number, lng: number) => {
    if (!leafletMap.current) return;
    const existing = markersRef.current.get(entryId);
    const entry = entries.find(e => e.id === entryId);
    const initials = entry?.patientInitials ?? '?';
    const icon = L.divIcon({
      className: '',
      html: `<div style="background:#6366f1;color:#fff;border-radius:50%;width:28px;height:28px;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:600;box-shadow:0 1px 3px rgba(0,0,0,.3)">${initials}</div>`,
      iconSize: [28, 28],
      iconAnchor: [14, 14],
    });
    if (existing) {
      existing.setLatLng([lat, lng]);
      existing.setIcon(icon);
    } else {
      const marker = L.marker([lat, lng], { icon })
        .addTo(leafletMap.current)
        .bindTooltip(initials)
        .on('click', () => setSelectedEntry(entryId));
      markersRef.current.set(entryId, marker);
    }
  };

  // Remove map pin when entry goes terminal
  useEffect(() => {
    entries.forEach(e => {
      if (['done', 'skipped', 'left'].includes(e.status)) {
        const marker = markersRef.current.get(e.id);
        if (marker && leafletMap.current) {
          leafletMap.current.removeLayer(marker);
          markersRef.current.delete(e.id);
        }
      }
    });
  }, [entries]);

  // --- Actions ---
  const handleCallNext = async () => {
    if (!queueID || calling) return;
    setCalling(true);
    try {
      await fetch(`/api/v1/queue/${queueID}/call-next`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ room: '' }),
      });
    } finally {
      setCalling(false);
    }
  };

  const handleUpdateStatus = async (entryId: string, status: string) => {
    if (!queueID) return;
    await fetch(`/api/v1/queue/${queueID}/entries/${entryId}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status }),
    });
  };

  const waitingCount = entries.filter(e => e.status === 'waiting').length;

  return (
    <AppShell title="Queue">
      <div className="flex gap-6 h-full">
        {/* Left panel — entry list */}
        <div className="flex-1 min-w-0 flex flex-col">
          {/* Header row */}
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <h2 className="text-lg font-semibold text-secondary-900">Today's Queue</h2>
              {waitingCount > 0 && (
                <span className="inline-flex items-center rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-700">
                  {waitingCount} waiting
                </span>
              )}
              {stats.avgWaitMinutes > 0 && (
                <span className="text-sm text-secondary-500">
                  ~{stats.avgWaitMinutes} min avg
                </span>
              )}
              {sseState === 'reconnecting' && (
                <span className="inline-flex items-center gap-1 rounded-full bg-orange-100 px-2.5 py-0.5 text-xs font-medium text-orange-700">
                  <svg className="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                  </svg>
                  Reconnecting…
                </span>
              )}
              {sseState === 'offline' && queueID && (
                <span className="inline-flex items-center rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-700">
                  Live updates paused
                </span>
              )}
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setShowMap(v => !v)}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                  showMap
                    ? 'bg-primary-100 text-primary-700'
                    : 'bg-secondary-100 text-secondary-600 hover:bg-secondary-200'
                }`}
              >
                {showMap ? 'Hide map' : 'Show map'}
              </button>
              <button
                onClick={handleCallNext}
                disabled={calling || waitingCount === 0}
                className="flex items-center gap-2 rounded-lg bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50 transition-colors"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M10.34 15.84c-.688-.06-1.386-.09-2.09-.09H7.5a4.5 4.5 0 1 1 0-9h.75c.704 0 1.402-.03 2.09-.09m0 9.18c.253.962.584 1.892.985 2.783.247.55.06 1.21-.463 1.511l-.657.38c-.551.318-1.26.117-1.527-.461a20.845 20.845 0 0 1-1.44-4.282m3.102.069a18.03 18.03 0 0 1-.59-4.59c0-1.586.205-3.124.59-4.59m0 9.18a23.848 23.848 0 0 1 8.835 2.535M10.34 6.66a23.847 23.847 0 0 0 8.835-2.535m0 0A23.74 23.74 0 0 0 18.795 3m.38 1.125a23.91 23.91 0 0 1 1.014 5.395m-1.014 8.855c-.118.38-.245.754-.38 1.125m.38-1.125a23.91 23.91 0 0 0 1.014-5.395m0-3.46c.495.413.811 1.035.811 1.73 0 .695-.316 1.317-.811 1.73m0-3.46a24.347 24.347 0 0 1 0 3.46" />
                </svg>
                {calling ? 'Calling…' : 'Call next'}
              </button>
            </div>
          </div>

          {loadError && (
            <p className="text-sm text-red-600 mb-4">{loadError}</p>
          )}

          {/* Entry list */}
          <div className="flex-1 overflow-y-auto space-y-2">
            {entries.length === 0 && (
              <div className="py-16 text-center text-secondary-400">
                <p className="text-base">No patients in the queue yet.</p>
                <p className="text-sm mt-1">Patients check in via the portal or reception.</p>
              </div>
            )}
            {entries.map(entry => {
              const badge = STATUS_BADGE[entry.status];
              const isSelected = selectedEntry === entry.id;
              return (
                <div
                  key={entry.id}
                  onClick={() => setSelectedEntry(isSelected ? null : entry.id)}
                  className={`rounded-xl border p-4 cursor-pointer transition-colors ${
                    isSelected
                      ? 'border-primary-400 bg-primary-50'
                      : 'border-secondary-200 bg-white hover:border-secondary-300'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="h-9 w-9 rounded-full bg-secondary-100 flex items-center justify-center">
                        <span className="text-sm font-semibold text-secondary-700">
                          {entry.patientInitials}
                        </span>
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="text-sm font-semibold text-secondary-900">
                            #{entry.position}
                          </span>
                          <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${badge.cls}`}>
                            {badge.label}
                          </span>
                          {entry.location && (
                            <span title="Patient has shared location" className="text-xs text-teal-600">
                              📍
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-secondary-500 mt-0.5">
                          Waited {waitLabel(entry.checkedInAt)}
                          {entry.roomHint && ` · ${entry.roomHint}`}
                        </p>
                      </div>
                    </div>

                    {/* Action buttons */}
                    <div className="flex items-center gap-1.5" onClick={e => e.stopPropagation()}>
                      {(entry.status === 'waiting' || entry.status === 'called') && (
                        <button
                          onClick={() => handleUpdateStatus(entry.id, 'done')}
                          className="px-2.5 py-1 rounded-lg text-xs font-medium bg-green-100 text-green-700 hover:bg-green-200 transition-colors"
                        >
                          Done
                        </button>
                      )}
                      {entry.status === 'waiting' && (
                        <button
                          onClick={() => handleUpdateStatus(entry.id, 'skipped')}
                          className="px-2.5 py-1 rounded-lg text-xs font-medium bg-gray-100 text-gray-600 hover:bg-gray-200 transition-colors"
                        >
                          Skip
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right panel — Leaflet map */}
        {showMap && (
          <div className="w-96 flex-shrink-0 rounded-xl border border-secondary-200 overflow-hidden">
            <div className="px-4 py-3 border-b border-secondary-100 flex items-center justify-between">
              <p className="text-sm font-semibold text-secondary-900">Patient locations</p>
              <p className="text-xs text-secondary-400">
                {entries.filter(e => e.location && !['done','skipped','left'].includes(e.status)).length} sharing
              </p>
            </div>
            <div ref={mapRef} className="h-[calc(100%-44px)]" />
          </div>
        )}
      </div>
    </AppShell>
  );
}
