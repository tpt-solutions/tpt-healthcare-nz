import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Romberg balance test via device gyroscope / orientation.
// Patient stands with feet together, arms at sides, phone in breast pocket.
// Two 30-second trials: eyes open, then eyes closed.
// Postural sway is quantified as total path length of orientation change.
// A Romberg ratio (sway_closed / sway_open) > 2 suggests vestibular or
// sensory ataxia.

export type BalanceStatus = "idle" | "measuring-open" | "rest" | "measuring-closed" | "done" | "error";

export interface BalanceTrial {
  swayDegrees: number; // total path length in degrees
  maxDeviationDeg: number;
  durationMs: number;
}

export interface BalanceResult {
  eyesOpen: BalanceTrial;
  eyesClosed: BalanceTrial;
  rombergRatio: number;
  interpretation: "normal" | "vestibular" | "sensory-ataxia" | "cerebellar";
  measuredAt: string;
}

export interface UseBalanceReturn {
  status: BalanceStatus;
  error: string | null;
  result: BalanceResult | null;
  progress: number;
  start: () => void;
  stop: () => void;
  toObservation: (r: BalanceResult, patientId?: string, encounterId?: string) => Observation;
}

const TRIAL_MS = 30_000;
const REST_MS  = 5_000;

function computeSway(alphas: number[], betas: number[]): { sway: number; maxDev: number } {
  if (alphas.length < 2) return { sway: 0, maxDev: 0 };
  let sway = 0, maxDev = 0;
  const baseA = alphas[0], baseB = betas[0];
  for (let i = 1; i < alphas.length; i++) {
    const da = alphas[i] - alphas[i - 1];
    const db = betas[i] - betas[i - 1];
    sway += Math.sqrt(da * da + db * db);
    const devFromBase = Math.sqrt((alphas[i] - baseA) ** 2 + (betas[i] - baseB) ** 2);
    if (devFromBase > maxDev) maxDev = devFromBase;
  }
  return { sway: Math.round(sway * 100) / 100, maxDev: Math.round(maxDev * 100) / 100 };
}

function interpret(ratio: number): BalanceResult["interpretation"] {
  if (ratio < 2)   return "normal";
  if (ratio < 3)   return "vestibular";
  if (ratio < 4)   return "sensory-ataxia";
  return "cerebellar";
}

export function useBalance(): UseBalanceReturn {
  const [status, setStatus]     = useState<BalanceStatus>("idle");
  const [error, setError]       = useState<string | null>(null);
  const [result, setResult]     = useState<BalanceResult | null>(null);
  const [progress, setProgress] = useState(0);

  const alphasRef    = useRef<number[]>([]);
  const betasRef     = useRef<number[]>([]);
  const openTrialRef = useRef<BalanceTrial | null>(null);
  const handlerRef   = useRef<((e: DeviceOrientationEvent) => void) | null>(null);
  const timerRef     = useRef<ReturnType<typeof setTimeout> | null>(null);
  const intervalRef  = useRef<ReturnType<typeof setInterval> | null>(null);
  const startTimeRef = useRef<number>(0);

  const cleanup = useCallback(() => {
    if (handlerRef.current) window.removeEventListener("deviceorientation", handlerRef.current);
    if (timerRef.current !== null)    clearTimeout(timerRef.current);
    if (intervalRef.current !== null) clearInterval(intervalRef.current);
    handlerRef.current = null;
    timerRef.current = null;
    intervalRef.current = null;
  }, []);

  const stop = useCallback(() => {
    cleanup();
    setStatus("idle");
    setProgress(0);
  }, [cleanup]);

  const runTrial = useCallback((phase: "open" | "closed", onComplete: (t: BalanceTrial) => void) => {
    alphasRef.current = [];
    betasRef.current  = [];
    startTimeRef.current = performance.now();

    const handler = (e: DeviceOrientationEvent) => {
      alphasRef.current.push(e.alpha ?? 0);
      betasRef.current.push(e.beta ?? 0);
    };
    handlerRef.current = handler;
    window.addEventListener("deviceorientation", handler);

    intervalRef.current = setInterval(() => {
      const elapsed = performance.now() - startTimeRef.current;
      setProgress(Math.min(100, Math.round((elapsed / TRIAL_MS) * 100)));
    }, 200);

    timerRef.current = setTimeout(() => {
      clearInterval(intervalRef.current!);
      window.removeEventListener("deviceorientation", handler);
      handlerRef.current = null;
      const { sway, maxDev } = computeSway(alphasRef.current, betasRef.current);
      onComplete({ swayDegrees: sway, maxDeviationDeg: maxDev, durationMs: TRIAL_MS });
    }, TRIAL_MS);
  }, []);

  const start = useCallback(() => {
    setError(null);
    setResult(null);
    setProgress(0);
    openTrialRef.current = null;

    const requestPermission = (DeviceOrientationEvent as unknown as { requestPermission?: () => Promise<string> }).requestPermission;

    const go = () => {
      setStatus("measuring-open");
      runTrial("open", openTrial => {
        openTrialRef.current = openTrial;
        setStatus("rest");
        setProgress(0);
        timerRef.current = setTimeout(() => {
          setStatus("measuring-closed");
          setProgress(0);
          runTrial("closed", closedTrial => {
            const ratio = openTrial.swayDegrees > 0 ? closedTrial.swayDegrees / openTrial.swayDegrees : 1;
            setResult({
              eyesOpen: openTrial,
              eyesClosed: closedTrial,
              rombergRatio: Math.round(ratio * 100) / 100,
              interpretation: interpret(ratio),
              measuredAt: new Date().toISOString(),
            });
            setProgress(100);
            setStatus("done");
          });
        }, REST_MS);
      });
    };

    if (typeof requestPermission === "function") {
      requestPermission()
        .then(state => {
          if (state === "granted") go();
          else { setError("Orientation sensor permission denied"); setStatus("error"); }
        })
        .catch(() => { setError("Orientation sensor unavailable"); setStatus("error"); });
    } else {
      go();
    }
  }, [runTrial]);

  const toObservation = useCallback(
    (r: BalanceResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://snomed.info/sct", code: "249857006", display: "Romberg test" }], text: "Romberg balance test" },
      effectiveDateTime: r.measuredAt,
      valueString: r.interpretation,
      component: [
        { code: { text: "Sway eyes open (deg)" }, valueQuantity: { value: r.eyesOpen.swayDegrees, unit: "deg" } },
        { code: { text: "Sway eyes closed (deg)" }, valueQuantity: { value: r.eyesClosed.swayDegrees, unit: "deg" } },
        { code: { text: "Romberg ratio" }, valueQuantity: { value: r.rombergRatio, unit: "ratio" } },
      ],
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, result, progress, start, stop, toObservation };
}
