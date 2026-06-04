import { useEffect, useRef, useState } from 'react';

export type SSEConnectionState = 'connected' | 'reconnecting' | 'offline';

const MAX_BACKOFF_MS = 30_000;

/**
 * Opens an SSE EventSource to `url` and automatically reconnects with exponential backoff
 * (1 s → 2 s → 4 s … capped at 30 s) on error.
 *
 * Reconnection pauses while the device is offline and resumes on the 'online' event.
 * The attempt counter resets to 0 on the first successful message, so a brief hiccup
 * doesn't permanently slow reconnects.
 *
 * @param url - SSE endpoint, or null to stay disconnected.
 * @param handlers - Map of SSE event-type → callback. Callbacks are always invoked
 *   with the latest reference (safe to define inline without extra useMemo/useCallback).
 * @returns Current connection state for UI indicators.
 */
export function useSSEStream(
  url: string | null,
  handlers: Record<string, (data: unknown) => void>
): SSEConnectionState {
  const [state, setState] = useState<SSEConnectionState>(url ? 'reconnecting' : 'offline');

  // Keep handlers up to date without re-triggering the effect
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  // Stable refs for cleanup
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

      es.onopen = () => {
        attemptRef.current = 0;
        setState('connected');
      };

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

      // Register handlers by type; look up the latest fn at dispatch time via handlersRef
      for (const type of Object.keys(handlersRef.current)) {
        es.addEventListener(type, (e) => {
          const fn = handlersRef.current[type];
          if (!fn) return;
          try {
            fn(JSON.parse((e as MessageEvent).data));
          } catch { /* malformed event — ignore */ }
        });
      }
    }

    connect();

    const onOnline = () => {
      if (!esRef.current) connect();
    };
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
