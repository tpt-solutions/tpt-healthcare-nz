import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";
import { useBluetooth } from "./useBluetooth.js";

// BLE Blood Pressure Monitor
// Implements Bluetooth GATT Blood Pressure Profile (0x1810):
//   Blood Pressure Measurement: 0x2A35
//   Intermediate Cuff Pressure: 0x2A36
//
// Compatible with OMRON BLE cuffs, iHealth Clear, Withings BPM Connect.
//
// Measurement characteristic (0x2A35) byte layout (per Bluetooth spec):
//   Byte 0:   Flags
//     bit 0 — unit (0=mmHg, 1=kPa)
//     bit 1 — timestamp present
//     bit 2 — pulse rate present
//     bit 3 — user ID present
//     bit 4 — measurement status present
//   Bytes 1–2: Systolic (SFLOAT, little-endian)
//   Bytes 3–4: Diastolic (SFLOAT)
//   Bytes 5–6: Mean Arterial Pressure (SFLOAT)
//   (optional: timestamp, pulse rate — see spec)

export interface BPReading {
  systolicMmHg: number;
  diastolicMmHg: number;
  meanArterialPressureMmHg: number;
  pulseRateBpm: number | null;
  classification: BPClassification;
  recordedAt: string;
}

export type BPClassification =
  | "optimal"
  | "normal"
  | "high-normal"
  | "grade-1-hypertension"
  | "grade-2-hypertension"
  | "grade-3-hypertension"
  | "isolated-systolic-hypertension";

export interface UseBloodPressureReturn {
  bleStatus: ReturnType<typeof useBluetooth>["status"];
  bleError: string | null;
  reading: BPReading | null;
  history: BPReading[];
  cuffPressure: number | null; // intermediate cuff reading during inflation
  connect: () => Promise<void>;
  disconnect: () => void;
  toObservation: (r: BPReading, patientId?: string, encounterId?: string) => Observation;
}

// SFLOAT decode (IEEE-11073 short float, 16-bit)
function sfloat(lo: number, hi: number): number {
  const raw = lo | (hi << 8);
  const mantissa = raw & 0x0FFF;
  const exponent = raw >> 12;
  const exp = exponent >= 8 ? exponent - 16 : exponent;
  const mant = mantissa >= 0x0800 ? mantissa - 0x1000 : mantissa;
  return mant * Math.pow(10, exp);
}

function classifyBP(sys: number, dia: number): BPClassification {
  if (sys < 120 && dia < 80)  return "optimal";
  if (sys < 130 && dia < 85)  return "normal";
  if (sys < 140 && dia < 90)  return "high-normal";
  if (sys >= 180 || dia >= 110) return "grade-3-hypertension";
  if (sys >= 160 || dia >= 100) return "grade-2-hypertension";
  if ((sys >= 140 && dia < 90)) return "isolated-systolic-hypertension";
  return "grade-1-hypertension";
}

function decodeBPM(data: DataView): { sys: number; dia: number; map: number; pulse: number | null } {
  const flags = data.getUint8(0);
  const isKpa = (flags & 0x01) !== 0;
  const hasPulse = (flags & 0x04) !== 0;

  let sys  = sfloat(data.getUint8(1), data.getUint8(2));
  let dia  = sfloat(data.getUint8(3), data.getUint8(4));
  let map  = sfloat(data.getUint8(5), data.getUint8(6));

  if (isKpa) { sys *= 7.50062; dia *= 7.50062; map *= 7.50062; }

  let byteOffset = 7;
  if (flags & 0x02) byteOffset += 7; // skip timestamp

  const pulse = hasPulse ? sfloat(data.getUint8(byteOffset), data.getUint8(byteOffset + 1)) : null;
  return { sys: Math.round(sys), dia: Math.round(dia), map: Math.round(map), pulse: pulse !== null ? Math.round(pulse) : null };
}

export function useBloodPressure(): UseBloodPressureReturn {
  const ble = useBluetooth();
  const [reading, setReading]         = useState<BPReading | null>(null);
  const [history, setHistory]         = useState<BPReading[]>([]);
  const [cuffPressure, setCuffPressure] = useState<number | null>(null);
  const bpCharRef   = useRef<BluetoothRemoteGATTCharacteristic | null>(null);
  const icpCharRef  = useRef<BluetoothRemoteGATTCharacteristic | null>(null);

  const onBPM = useCallback((event: Event) => {
    const char = event.target as BluetoothRemoteGATTCharacteristic;
    const { sys, dia, map, pulse } = decodeBPM(char.value!);
    const r: BPReading = {
      systolicMmHg: sys,
      diastolicMmHg: dia,
      meanArterialPressureMmHg: map,
      pulseRateBpm: pulse,
      classification: classifyBP(sys, dia),
      recordedAt: new Date().toISOString(),
    };
    setReading(r);
    setHistory(h => [...h.slice(-99), r]);
    setCuffPressure(null);
  }, []);

  const onICP = useCallback((event: Event) => {
    const char = event.target as BluetoothRemoteGATTCharacteristic;
    const flags = char.value!.getUint8(0);
    const isKpa = (flags & 0x01) !== 0;
    let cuff = sfloat(char.value!.getUint8(1), char.value!.getUint8(2));
    if (isKpa) cuff *= 7.50062;
    setCuffPressure(Math.round(cuff));
  }, []);

  const connect = useCallback(async () => {
    setReading(null);
    setHistory([]);
    setCuffPressure(null);

    const server = await ble.connect(
      [{ services: ["blood_pressure"] }],
      ["blood_pressure"]
    );
    if (!server) return;

    try {
      const bpChar = await ble.getCharacteristic("blood_pressure", "blood_pressure_measurement");
      bpCharRef.current = bpChar;
      bpChar.addEventListener("characteristicvaluechanged", onBPM);
      await bpChar.startNotifications();
    } catch { /* ignore */ }

    try {
      const icpChar = await ble.getCharacteristic("blood_pressure", "intermediate_cuff_pressure");
      icpCharRef.current = icpChar;
      icpChar.addEventListener("characteristicvaluechanged", onICP);
      await icpChar.startNotifications();
    } catch { /* device may not stream ICP */ }
  }, [ble, onBPM, onICP]);

  const disconnect = useCallback(() => {
    bpCharRef.current?.removeEventListener("characteristicvaluechanged", onBPM);
    icpCharRef.current?.removeEventListener("characteristicvaluechanged", onICP);
    ble.disconnect();
  }, [ble, onBPM, onICP]);

  const toObservation = useCallback(
    (r: BPReading, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "vital-signs" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "55284-4", display: "Blood pressure systolic and diastolic" }], text: "Blood pressure" },
      effectiveDateTime: r.recordedAt,
      component: [
        { code: { coding: [{ system: "http://loinc.org", code: "8480-6", display: "Systolic blood pressure" }] }, valueQuantity: { value: r.systolicMmHg, unit: "mmHg", system: "http://unitsofmeasure.org", code: "mm[Hg]" } },
        { code: { coding: [{ system: "http://loinc.org", code: "8462-4", display: "Diastolic blood pressure" }] }, valueQuantity: { value: r.diastolicMmHg, unit: "mmHg", system: "http://unitsofmeasure.org", code: "mm[Hg]" } },
        { code: { coding: [{ system: "http://loinc.org", code: "8478-0", display: "Mean blood pressure" }] }, valueQuantity: { value: r.meanArterialPressureMmHg, unit: "mmHg", system: "http://unitsofmeasure.org", code: "mm[Hg]" } },
        ...(r.pulseRateBpm !== null ? [{ code: { coding: [{ system: "http://loinc.org", code: "8867-4", display: "Heart rate" }] }, valueQuantity: { value: r.pulseRateBpm, unit: "/min", system: "http://unitsofmeasure.org", code: "/min" } }] : []),
      ],
      interpretation: [{ text: r.classification }],
      method: { text: "BLE sphygmomanometer" },
      device: { display: ble.device?.name ?? "BLE BP cuff" },
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    [ble.device]
  );

  return { bleStatus: ble.status, bleError: ble.error, reading, history, cuffPressure, connect, disconnect, toObservation };
}
