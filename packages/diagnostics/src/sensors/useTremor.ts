import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Tremor assessment via device accelerometer.
// Resting tremor (3–6 Hz) is associated with Parkinson's disease.
// Action/postural tremor (6–12 Hz) is associated with essential tremor.
// The patient holds the phone in outstretched hand (action) or rests it
// on the back of their hand (resting) for the measurement period.

export type TremorStatus = "idle" | "measuring" | "done" | "error";
export type TremorKind = "resting" | "action";

export interface TremorResult {
  kind: TremorKind;
  dominantFreqHz: number;
  powerSpectralDensity: number; // normalised amplitude — higher = more tremor
  classification: "normal" | "mild" | "moderate" | "severe";
  measuredAt: string;
}

export interface UseTremorReturn {
  status: TremorStatus;
  error: string | null;
  result: TremorResult | null;
  progress: number;
  start: (kind?: TremorKind) => void;
  stop: () => void;
  toObservation: (r: TremorResult, patientId?: string, encounterId?: string) => Observation;
}

const SAMPLE_DURATION_MS = 10_000;
const SAMPLE_RATE_HZ = 60;
const SAMPLE_INTERVAL_MS = 1000 / SAMPLE_RATE_HZ;

// Tremor frequency bands (Hz)
const RESTING_BAND  = [3, 6] as const;
const ACTION_BAND   = [5, 12] as const;

// PSD thresholds (empirical — calibrate against clinical data)
const PSD_MILD     = 0.02;
const PSD_MODERATE = 0.08;
const PSD_SEVERE   = 0.25;

function bandpower(samples: number[], fs: number, fLow: number, fHigh: number): { freq: number; power: number } {
  const n = samples.length;
  const mean = samples.reduce((a, b) => a + b, 0) / n;
  const x = samples.map(v => v - mean);
  let bestFreq = 0, bestPow = 0;
  for (let k = 1; k < n / 2; k++) {
    const freq = (k * fs) / n;
    if (freq < fLow || freq > fHigh) continue;
    let re = 0, im = 0;
    for (let i = 0; i < n; i++) {
      const phi = (2 * Math.PI * k * i) / n;
      re += x[i] * Math.cos(phi);
      im -= x[i] * Math.sin(phi);
    }
    const pow = (re * re + im * im) / (n * n);
    if (pow > bestPow) { bestPow = pow; bestFreq = freq; }
  }
  return { freq: bestFreq, power: bestPow };
}

function classify(psd: number): TremorResult["classification"] {
  if (psd < PSD_MILD)     return "normal";
  if (psd < PSD_MODERATE) return "mild";
  if (psd < PSD_SEVERE)   return "moderate";
  return "severe";
}

export function useTremor(): UseTremorReturn {
  const [status, setStatus]   = useState<TremorStatus>("idle");
  const [error, setError]     = useState<string | null>(null);
  const [result, setResult]   = useState<TremorResult | null>(null);
  const [progress, setProgress] = useState(0);

  const samplesRef   = useRef<number[]>([]);
  const timerRef     = useRef<ReturnType<typeof setInterval> | null>(null);
  const startTimeRef = useRef<number>(0);
  const kindRef      = useRef<TremorKind>("action");
  const handlerRef   = useRef<((e: DeviceMotionEvent) => void) | null>(null);

  const stop = useCallback(() => {
    if (timerRef.current !== null) clearInterval(timerRef.current);
    if (handlerRef.current) window.removeEventListener("devicemotion", handlerRef.current);
    timerRef.current = null;
    handlerRef.current = null;
    setStatus("idle");
    setProgress(0);
  }, []);

  const start = useCallback((kind: TremorKind = "action") => {
    kindRef.current = kind;
    setError(null);
    setResult(null);
    setProgress(0);
    samplesRef.current = [];
    startTimeRef.current = performance.now();

    const handler = (e: DeviceMotionEvent) => {
      const acc = e.accelerationIncludingGravity;
      if (!acc) return;
      const mag = Math.sqrt((acc.x ?? 0) ** 2 + (acc.y ?? 0) ** 2 + (acc.z ?? 0) ** 2);
      samplesRef.current.push(mag);
    };
    handlerRef.current = handler;

    // iOS 13+ requires permission
    if (typeof (DeviceMotionEvent as unknown as { requestPermission?: () => Promise<string> }).requestPermission === "function") {
      (DeviceMotionEvent as unknown as { requestPermission: () => Promise<string> })
        .requestPermission()
        .then(state => {
          if (state === "granted") {
            window.addEventListener("devicemotion", handler);
          } else {
            setError("Motion sensor permission denied");
            setStatus("error");
          }
        })
        .catch(() => {
          setError("Motion sensor unavailable");
          setStatus("error");
        });
    } else {
      window.addEventListener("devicemotion", handler);
    }

    setStatus("measuring");

    timerRef.current = setInterval(() => {
      const elapsed = performance.now() - startTimeRef.current;
      setProgress(Math.min(100, Math.round((elapsed / SAMPLE_DURATION_MS) * 100)));

      if (elapsed >= SAMPLE_DURATION_MS) {
        clearInterval(timerRef.current!);
        window.removeEventListener("devicemotion", handler);
        timerRef.current = null;
        handlerRef.current = null;

        const band = kindRef.current === "resting" ? RESTING_BAND : ACTION_BAND;
        const { freq, power } = bandpower(samplesRef.current, SAMPLE_RATE_HZ, band[0], band[1]);

        setResult({
          kind: kindRef.current,
          dominantFreqHz: Math.round(freq * 10) / 10,
          powerSpectralDensity: Math.round(power * 10000) / 10000,
          classification: classify(power),
          measuredAt: new Date().toISOString(),
        });
        setProgress(100);
        setStatus("done");
      }
    }, SAMPLE_INTERVAL_MS);
  }, []);

  const toObservation = useCallback(
    (r: TremorResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://snomed.info/sct", code: "26079004", display: "Tremor" }], text: "Tremor assessment" },
      effectiveDateTime: r.measuredAt,
      valueCodeableConcept: {
        coding: [{ system: "http://snomed.info/sct", code: r.classification === "normal" ? "17621005" : "263654008", display: r.classification }],
        text: r.classification,
      },
      component: [
        { code: { text: "Dominant tremor frequency" }, valueQuantity: { value: r.dominantFreqHz, unit: "Hz" } },
        { code: { text: "Power spectral density" }, valueQuantity: { value: r.powerSpectralDensity, unit: "m2/s4/Hz" } },
        { code: { text: "Tremor kind" }, valueString: r.kind },
      ],
      method: { text: `Smartphone accelerometer — ${r.kind} tremor test` },
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, result, progress, start, stop, toObservation };
}
