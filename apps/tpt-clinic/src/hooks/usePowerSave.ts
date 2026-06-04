import { useEffect, useState } from 'react';

/**
 * Monitors battery level and network quality, signalling the service worker to enter
 * power-save mode when conditions are degraded.
 *
 * Power-save mode is triggered when ANY of the following is true:
 *   - Battery level < 20% and not charging (device may be on UPS/battery backup)
 *   - Network RTT > 800 ms (high-latency link — don't hammer background fetches)
 *   - Network downlink < 1 Mbps (constrained bandwidth)
 *
 * In power-save mode the SW reduces background cache refreshes and SSE heartbeat frequency.
 *
 * Returns { isPowerSave } so callers can gate expensive operations.
 */
export function usePowerSave(): { isPowerSave: boolean } {
  const [isPowerSave, setIsPowerSave] = useState(false);

  useEffect(() => {
    let battery: BatteryManager | null = null;

    const postToSW = (on: boolean) => {
      navigator.serviceWorker?.ready.then((reg) => {
        reg.active?.postMessage({ type: on ? 'POWER_SAVE_ON' : 'POWER_SAVE_OFF' });
      });
    };

    const evaluate = () => {
      const batteryLow = battery ? battery.level < 0.2 && !battery.charging : false;

      const conn = (navigator as unknown as { connection?: NetworkInformation }).connection;
      const highLatency = conn ? conn.rtt > 800 : false;
      const lowBandwidth = conn ? conn.downlink < 1 : false;

      const on = batteryLow || highLatency || lowBandwidth;
      setIsPowerSave(on);
      postToSW(on);
    };

    // Battery Status API (still supported in Chrome/Android WebView)
    if ('getBattery' in navigator) {
      (navigator as unknown as { getBattery(): Promise<BatteryManager> })
        .getBattery()
        .then((b) => {
          battery = b;
          b.addEventListener('levelchange', evaluate);
          b.addEventListener('chargingchange', evaluate);
          evaluate();
        })
        .catch(() => { /* API unavailable */ });
    }

    // Network Information API
    const conn = (navigator as unknown as { connection?: NetworkInformation }).connection;
    if (conn) {
      conn.addEventListener('change', evaluate);
      evaluate();
    }

    return () => {
      if (battery) {
        battery.removeEventListener('levelchange', evaluate);
        battery.removeEventListener('chargingchange', evaluate);
      }
      conn?.removeEventListener('change', evaluate);
    };
  }, []);

  return { isPowerSave };
}

interface BatteryManager extends EventTarget {
  level: number;
  charging: boolean;
  addEventListener(type: 'levelchange' | 'chargingchange', listener: EventListener): void;
  removeEventListener(type: 'levelchange' | 'chargingchange', listener: EventListener): void;
}

interface NetworkInformation extends EventTarget {
  rtt: number;
  downlink: number;
  addEventListener(type: 'change', listener: EventListener): void;
  removeEventListener(type: 'change', listener: EventListener): void;
}
