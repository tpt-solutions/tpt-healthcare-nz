import { useEffect, useRef, useState } from 'react';

export type SSEConnectionState = 'connected' | 'reconnecting' | 'offline';

const MAX_BACKOFF_MS = 30_000;

/**
 * Opens an SSE EventSource with exponential-backoff reconnection (1s → 2s → … → 30s).
 * Pauses while offline; resumes on the 'online' event.
 * Handlers are always called with their latest reference — safe to define inline.
 */
export function useSSEStream(
  url: string | null,
  handlers: Record<string, (data: unknown) => void>
): SSEConnectionState {
  const [state, setState] = useState<SSEConnectionState>(url ? 'reconnecting' : 'offline');
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  const esRef = useRef<EventSource | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const attemptRef = useRef(0);
  const activeUrlRef = useRef<string | null>(null);

  useEffect(() => {
    if (!url) {
      setState('offline');
      return;
    }

    activeUrlRef.current = url;
    attemptRef.current = 0;

    function connect() {
      if (!activeUrlRef.current || !navigator.onLine) {
        setState('offline');
        return;
      }

      const es = new EventSource(activeUrlRef.current);
      esRef.current = es;
      setState('reconnecting');

      es.onopen = () => { attemptRef.current = 0; setState('connected'); };

      es.onerror = () => {
        es.close();
        esRef.current = null;

        if (!navigator.onLine || !activeUrlRef.current) {
          setState('offline');
          return;
        }

        setState('reconnecting');
        const delay = Math.min(1_000 * Math.pow(2, attemptRef.current), MAX_BACKOFF_MS);
        attemptRef.current += 1;
        timerRef.current = setTimeout(connect, delay);
      };

      for (const type of Object.keys(handlersRef.current)) {
        es.addEventListener(type, (e) => {
          const fn = handlersRef.current[type];
          if (!fn) return;
          try { fn(JSON.parse((e as MessageEvent).data)); } catch { /* ignore */ }
        });
      }
    }

    connect();

    const onOnline = () => { if (!esRef.current) connect(); };
    const onOffline = () => {
      esRef.current?.close();
      esRef.current = null;
      if (timerRef.current) { clearTimeout(timerRef.current); timerRef.current = null; }
      setState('offline');
    };

    window.addEventListener('online', onOnline);
    window.addEventListener('offline', onOffline);

    return () => {
      activeUrlRef.current = null;
      esRef.current?.close();
      esRef.current = null;
      if (timerRef.current) { clearTimeout(timerRef.current); timerRef.current = null; }
      window.removeEventListener('online', onOnline);
      window.removeEventListener('offline', onOffline);
    };
  }, [url]);

  return state;
}
