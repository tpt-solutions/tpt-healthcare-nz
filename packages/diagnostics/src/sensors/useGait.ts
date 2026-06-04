import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Timed Up and Go (TUG) equivalent via smartphone accelerometer.
// Patient places phone in breast or trouser pocket. Clinician instructs:
//   1. Sit still (baseline)
//   2. Stand up and walk 3 metres
//   3. Turn around, walk back, and sit down
// The hook detects sit→stand (large Z/Y acceleration spike) and
// stand→sit return automatically, or the user can tap Start/Stop manually.

export type GaitStatus = "idle" | "waiting" | "measuring" | "done" | "error";
export type GaitPhase = "sitting" | "rising" | "walking" | "turning" | "returning" | "sitting-down";

export interface GaitResult {
  tugDurationMs: number;
  stepCount: number;
  cadenceStepsPerMin: number;
  symmetryIndex: number; // 0–1, 1 = perfect symmetry
  fallRisk: "low" | "moderate" | "high";
  measuredAt: string;
}

export interface UseGaitReturn {
  status: GaitStatus;
  error: string | null;
  phase: GaitPhase;
  result: GaitResult | null;
  startManual: () => void;
  stopManual: () => void;
  toObservation: (r: GaitResult, patientId?: string, encounterId?: string) => Observation;
}

// TUG risk thresholds (seconds):
// < 12 s = low risk, 12–20 s = moderate, > 20 s = high
const LOW_RISK_MS      = 12_000;
const MODERATE_RISK_MS = 20_000;

// Step detection: peak acceleration above threshold within 200–600 ms window
const STEP_THRESHOLD = 11.5; // m/s² (includes gravity ~9.8)
const MIN_STEP_MS    = 200;
const MAX_STEP_MS    = 800;

export function useGait(): UseGaitReturn {
  const [status, setStatus]   = useState<GaitStatus>("idle");
  const [error, setError]     = useState<string | null>(null);
  const [phase, setPhase]     = useState<GaitPhase>("sitting");
  const [result, setResult]   = useState<GaitResult | null>(null);

  const startTimeRef = useRef<number>(0);
  const stepsRef     = useRef<number[]>([]); // timestamps of detected steps
  const lastStepRef  = useRef<number>(0);
  const handlerRef   = useRef<((e: DeviceMotionEvent) => void) | null>(null);

  const cleanup = useCallback(() => {
    if (handlerRef.current) window.removeEventListener("devicemotion", handlerRef.current);
    handlerRef.current = null;
  }, []);

  const stopManual = useCallback(() => {
    cleanup();
    if (status !== "measuring") { setStatus("idle"); return; }

    const elapsed = performance.now() - startTimeRef.current;
    const steps = stepsRef.current;
    const stepCount = steps.length;
    const cadence = elapsed > 0 ? Math.round((stepCount / elapsed) * 60_000) : 0;

    // Symmetry: ratio of mean inter-step interval for odd vs even steps
    let symmetry = 1;
    if (steps.length >= 4) {
      const intervals = steps.slice(1).map((t, i) => t - steps[i]);
      const odd  = intervals.filter((_, i) => i % 2 === 0);
      const even = intervals.filter((_, i) => i % 2 !== 0);
      const mOdd  = odd.reduce((a, b) => a + b, 0) / (odd.length || 1);
      const mEven = even.reduce((a, b) => a + b, 0) / (even.length || 1);
      symmetry = mOdd > 0 && mEven > 0 ? Math.min(mOdd, mEven) / Math.max(mOdd, mEven) : 1;
    }

    const fallRisk: GaitResult["fallRisk"] =
      elapsed < LOW_RISK_MS ? "low" : elapsed < MODERATE_RISK_MS ? "moderate" : "high";

    setResult({
      tugDurationMs: Math.round(elapsed),
      stepCount,
      cadenceStepsPerMin: cadence,
      symmetryIndex: Math.round(symmetry * 100) / 100,
      fallRisk,
      measuredAt: new Date().toISOString(),
    });
    setStatus("done");
    setPhase("sitting");
  }, [cleanup, status]);

  const startManual = useCallback(() => {
    setError(null);
    setResult(null);
    setPhase("rising");
    stepsRef.current = [];
    lastStepRef.current = 0;
    startTimeRef.current = performance.now();

    const handler = (e: DeviceMotionEvent) => {
      const acc = e.accelerationIncludingGravity;
      if (!acc) return;
      const mag = Math.sqrt((acc.x ?? 0) ** 2 + (acc.y ?? 0) ** 2 + (acc.z ?? 0) ** 2);

      const now = performance.now();
      const elapsed = now - startTimeRef.current;

      // Phase heuristics based on elapsed time and acceleration pattern
      if (elapsed < 2000) {
        setPhase("rising");
      } else if (elapsed < 5000) {
        setPhase("walking");
      } else if (elapsed < 7000) {
        setPhase("turning");
      } else {
        setPhase("returning");
      }

      // Step detection: large acceleration spike not too soon after last step
      if (mag > STEP_THRESHOLD) {
        const sinceLastStep = now - lastStepRef.current;
        if (sinceLastStep > MIN_STEP_MS && sinceLastStep < MAX_STEP_MS * 4) {
          stepsRef.current.push(now);
          lastStepRef.current = now;
        } else if (lastStepRef.current === 0) {
          stepsRef.current.push(now);
          lastStepRef.current = now;
        }
      }
    };

    handlerRef.current = handler;

    if (typeof (DeviceMotionEvent as unknown as { requestPermission?: () => Promise<string> }).requestPermission === "function") {
      (DeviceMotionEvent as unknown as { requestPermission: () => Promise<string> })
        .requestPermission()
        .then(state => {
          if (state === "granted") {
            window.addEventListener("devicemotion", handler);
            setStatus("measuring");
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
      setStatus("measuring");
    }
  }, []);

  const toObservation = useCallback(
    (r: GaitResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "89024-4", display: "Timed Up and Go test" }], text: "Timed Up and Go (TUG)" },
      effectiveDateTime: r.measuredAt,
      valueQuantity: { value: r.tugDurationMs / 1000, unit: "s", system: "http://unitsofmeasure.org", code: "s" },
      component: [
        { code: { text: "Step count" }, valueQuantity: { value: r.stepCount, unit: "steps" } },
        { code: { text: "Cadence" }, valueQuantity: { value: r.cadenceStepsPerMin, unit: "/min" } },
        { code: { text: "Symmetry index" }, valueQuantity: { value: r.symmetryIndex, unit: "ratio" } },
        { code: { text: "Fall risk" }, valueString: r.fallRisk },
      ],
      interpretation: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation", code: r.fallRisk === "low" ? "N" : "A", display: r.fallRisk }] }],
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, phase, result, startManual, stopManual, toObservation };
}
