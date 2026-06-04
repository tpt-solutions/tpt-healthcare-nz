import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";
import { useBluetooth } from "./useBluetooth.js";

// BLE Pulse Oximeter — reads SpO2 and heart rate from Bluetooth LE devices
// that implement the standard Bluetooth GATT Health Thermometer / Pulse Oximeter
// profiles (e.g. Wellue O2Ring, Nonin 3230, iHealth Air).
//
// Standard GATT service/characteristic UUIDs:
//   Heart Rate:          0x180D / 0x2A37 (Heart Rate Measurement)
//   Pulse Oximeter:      0x1822 / 0x2A5F (PLX Spot-Check Measurement)
//                        0x1822 / 0x2A60 (PLX Continuous Measurement)
//
// Byte decoding follows Bluetooth Assigned Numbers spec for these characteristics.

export interface OximeterReading {
  spO2Percent: number;
  heartRateBpm: number;
  recordedAt: string;
}

export interface UseOximeterReturn {
  bleStatus: ReturnType<typeof useBluetooth>["status"];
  bleError: string | null;
  reading: OximeterReading | null;
  history: OximeterReading[];
  connect: () => Promise<void>;
  disconnect: () => void;
  toObservations: (r: OximeterReading, patientId?: string, encounterId?: string) => Observation[];
}

// Heart Rate Measurement (0x2A37) decoder
function decodeHRM(data: DataView): { heartRate: number } {
  const flags = data.getUint8(0);
  const is16bit = (flags & 0x01) !== 0;
  const heartRate = is16bit ? data.getUint16(1, true) : data.getUint8(1);
  return { heartRate };
}

// PLX Spot-Check (0x2A5F) decoder — flags byte + SpO2 + PR
function decodePLX(data: DataView): { spO2: number; pr: number } {
  // Flags at byte 0; SpO2 at bytes 1-2 (SFLOAT), PR at bytes 3-4 (SFLOAT)
  const spO2 = data.getUint16(1, true) / 10;
  const pr   = data.getUint16(3, true) / 10;
  return { spO2: Math.round(spO2), pr: Math.round(pr) };
}

export function useOximeter(): UseOximeterReturn {
  const ble = useBluetooth();
  const [reading, setReading]   = useState<OximeterReading | null>(null);
  const [history, setHistory]   = useState<OximeterReading[]>([]);
  const hrmCharRef = useRef<BluetoothRemoteGATTCharacteristic | null>(null);
  const plxCharRef = useRef<BluetoothRemoteGATTCharacteristic | null>(null);
  const latestSpO2Ref = useRef<number>(0);
  const latestHRRef   = useRef<number>(0);

  const onHRM = useCallback((event: Event) => {
    const char = event.target as BluetoothRemoteGATTCharacteristic;
    const { heartRate } = decodeHRM(char.value!);
    latestHRRef.current = heartRate;
    if (latestSpO2Ref.current > 0) {
      const r: OximeterReading = { spO2Percent: latestSpO2Ref.current, heartRateBpm: heartRate, recordedAt: new Date().toISOString() };
      setReading(r);
      setHistory(h => [...h.slice(-99), r]);
    }
  }, []);

  const onPLX = useCallback((event: Event) => {
    const char = event.target as BluetoothRemoteGATTCharacteristic;
    const { spO2, pr } = decodePLX(char.value!);
    latestSpO2Ref.current = spO2;
    latestHRRef.current   = pr;
    const r: OximeterReading = { spO2Percent: spO2, heartRateBpm: pr, recordedAt: new Date().toISOString() };
    setReading(r);
    setHistory(h => [...h.slice(-99), r]);
  }, []);

  const connect = useCallback(async () => {
    setReading(null);
    setHistory([]);
    latestSpO2Ref.current = 0;
    latestHRRef.current   = 0;

    const server = await ble.connect(
      [{ services: ["heart_rate"] }, { services: [0x1822] }],
      ["heart_rate", 0x1822]
    );
    if (!server) return;

    // Try Heart Rate profile
    try {
      const hrChar = await ble.getCharacteristic("heart_rate", "heart_rate_measurement");
      hrmCharRef.current = hrChar;
      hrChar.addEventListener("characteristicvaluechanged", onHRM);
      await hrChar.startNotifications();
    } catch { /* device may not support HR profile */ }

    // Try PLX profile
    try {
      const plxChar = await ble.getCharacteristic(0x1822.toString(16), 0x2A60.toString(16).padStart(4, "0"));
      plxCharRef.current = plxChar;
      plxChar.addEventListener("characteristicvaluechanged", onPLX);
      await plxChar.startNotifications();
    } catch { /* device may not support PLX continuous */ }
  }, [ble, onHRM, onPLX]);

  const disconnect = useCallback(() => {
    hrmCharRef.current?.removeEventListener("characteristicvaluechanged", onHRM);
    plxCharRef.current?.removeEventListener("characteristicvaluechanged", onPLX);
    ble.disconnect();
  }, [ble, onHRM, onPLX]);

  const toObservations = useCallback(
    (r: OximeterReading, patientId?: string, encounterId?: string): Observation[] => {
      const subject = patientId ? { subject: { reference: `Patient/${patientId}` } } : {};
      const encounter = encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {};
      return [
        {
          resourceType: "Observation",
          status: "final",
          category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "vital-signs" }] }],
          code: { coding: [{ system: "http://loinc.org", code: "2708-6", display: "Oxygen saturation" }], text: "SpO2" },
          effectiveDateTime: r.recordedAt,
          valueQuantity: { value: r.spO2Percent, unit: "%", system: "http://unitsofmeasure.org", code: "%" },
          method: { coding: [{ system: "http://snomed.info/sct", code: "448703006", display: "Pulse oximetry" }] },
          device: { display: ble.device?.name ?? "BLE pulse oximeter" },
          ...subject, ...encounter,
        },
        {
          resourceType: "Observation",
          status: "final",
          category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "vital-signs" }] }],
          code: { coding: [{ system: "http://loinc.org", code: "8867-4", display: "Heart rate" }], text: "Heart rate" },
          effectiveDateTime: r.recordedAt,
          valueQuantity: { value: r.heartRateBpm, unit: "/min", system: "http://unitsofmeasure.org", code: "/min" },
          method: { coding: [{ system: "http://snomed.info/sct", code: "448703006", display: "Pulse oximetry" }] },
          device: { display: ble.device?.name ?? "BLE pulse oximeter" },
          ...subject, ...encounter,
        },
      ];
    },
    [ble.device]
  );

  return { bleStatus: ble.status, bleError: ble.error, reading, history, connect, disconnect, toObservations };
}
