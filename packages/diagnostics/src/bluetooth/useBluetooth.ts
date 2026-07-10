import { useCallback, useRef, useState } from "react";

// Generic Web Bluetooth device scanner and connector.
// Specialised hooks (useOximeter, useBloodPressure) build on top of this.

export type BluetoothStatus = "idle" | "scanning" | "connecting" | "connected" | "disconnected" | "error" | "unsupported";

export interface BLEDeviceInfo {
  id: string;
  name: string | undefined;
}

export interface UseBluetoothReturn {
  status: BluetoothStatus;
  error: string | null;
  device: BLEDeviceInfo | null;
  getCharacteristic: (serviceUUID: string, charUUID: string) => Promise<BluetoothRemoteGATTCharacteristic>;
  connect: (filters: BluetoothLEScanFilter[], optionalServices?: BluetoothServiceUUID[]) => Promise<BluetoothRemoteGATTServer | null>;
  disconnect: () => void;
}

export function useBluetooth(): UseBluetoothReturn {
  const [status, setStatus] = useState<BluetoothStatus>(() =>
    typeof navigator !== "undefined" && "bluetooth" in navigator ? "idle" : "unsupported"
  );
  const [error, setError]   = useState<string | null>(null);
  const [device, setDevice] = useState<BLEDeviceInfo | null>(null);
  const gattRef = useRef<BluetoothRemoteGATTServer | null>(null);
  const bleDevRef = useRef<BluetoothDevice | null>(null);

  const disconnect = useCallback(() => {
    gattRef.current?.disconnect();
    bleDevRef.current?.removeEventListener("gattserverdisconnected", () => {});
    gattRef.current = null;
    bleDevRef.current = null;
    setStatus("disconnected");
    setDevice(null);
  }, []);

  const connect = useCallback(async (
    filters: BluetoothLEScanFilter[],
    optionalServices: BluetoothServiceUUID[] = []
  ): Promise<BluetoothRemoteGATTServer | null> => {
    if (status === "unsupported") {
      setError("Web Bluetooth not supported in this browser");
      return null;
    }
    setError(null);
    setStatus("scanning");
    try {
      const bleDevice = await navigator.bluetooth.requestDevice({ filters, optionalServices });
      bleDevRef.current = bleDevice;
      setStatus("connecting");
      setDevice({ id: bleDevice.id, name: bleDevice.name });

      bleDevice.addEventListener("gattserverdisconnected", () => {
        setStatus("disconnected");
        gattRef.current = null;
      });

      const server = await bleDevice.gatt!.connect();
      gattRef.current = server;
      setStatus("connected");
      return server;
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Bluetooth connection failed";
      setError(msg);
      setStatus("error");
      return null;
    }
  }, [status]);

  const getCharacteristic = useCallback(async (
    serviceUUID: string,
    charUUID: string
  ): Promise<BluetoothRemoteGATTCharacteristic> => {
    if (!gattRef.current) throw new Error("Not connected");
    const service = await gattRef.current.getPrimaryService(serviceUUID);
    return service.getCharacteristic(charUUID);
  }, []);

  return { status, error, device, getCharacteristic, connect, disconnect };
}
