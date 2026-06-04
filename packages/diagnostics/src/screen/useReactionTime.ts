import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Reaction time / fine motor assessment via touch events.
// Displays a target at random screen positions; patient taps as fast as possible.
// Measures: simple reaction time (mean ms), coefficient of variation (consistency),
// and miss rate (spatial inaccuracy → fine motor proxy).
// Useful for cognitive screening and neurological baseline.

export type ReactionStatus = "idle" | "waiting" | "ready" | "done" | "error";

export interface ReactionTrial {
  targetX: number;       // 0–1 relative to container
  targetY: number;
  displayedAt: number;   // performance.now() timestamp
  respondedAt: number | null;
  reactionMs: number | null;
  hitDistance: number;   // pixels from tap to target centre
  hit: boolean;
}

export interface ReactionResult {
  meanReactionMs: number;
  medianReactionMs: number;
  cvPercent: number;          // coefficient of variation (lower = more consistent)
  missRate: number;           // 0–1
  cognitiveScreen: "normal" | "borderline" | "impaired";
  trials: ReactionTrial[];
  measuredAt: string;
}

export interface UseReactionTimeReturn {
  status: ReactionStatus;
  currentTrial: number;
  totalTrials: number;
  targetX: number | null;      // 0–1 relative position for caller to render
  targetY: number | null;
  result: ReactionResult | null;
  start: (trials?: number) => void;
  recordTap: (tapX: number, tapY: number, containerW: number, containerH: number) => void;
  toObservation: (r: ReactionResult, patientId?: string, encounterId?: string) => Observation;
}

const TARGET_RADIUS_PX = 40;
// Reaction time thresholds (ms) — rough normative values for adults:
const NORMAL_MEAN_MS     = 400;
const BORDERLINE_MEAN_MS = 600;

function median(arr: number[]): number {
  const sorted = [...arr].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 0 ? (sorted[mid - 1] + sorted[mid]) / 2 : sorted[mid];
}

function cv(arr: number[]): number {
  if (arr.length < 2) return 0;
  const mean = arr.reduce((a, b) => a + b, 0) / arr.length;
  const variance = arr.reduce((sum, v) => sum + (v - mean) ** 2, 0) / arr.length;
  return mean > 0 ? (Math.sqrt(variance) / mean) * 100 : 0;
}

export function useReactionTime(): UseReactionTimeReturn {
  const [status, setStatus]     = useState<ReactionStatus>("idle");
  const [currentTrial, setCurrent] = useState(0);
  const [totalTrials, setTotal]   = useState(10);
  const [targetX, setTargetX]   = useState<number | null>(null);
  const [targetY, setTargetY]   = useState<number | null>(null);
  const [result, setResult]     = useState<ReactionResult | null>(null);

  const trialsRef   = useRef<ReactionTrial[]>([]);
  const idxRef      = useRef(0);
  const timerRef    = useRef<ReturnType<typeof setTimeout> | null>(null);
  const displayedAtRef = useRef<number>(0);
  const pendingRef  = useRef(false);

  const showNext = useCallback(() => {
    // Random delay 800–2500 ms before showing target (prevent anticipation)
    const delay = 800 + Math.random() * 1700;
    setStatus("waiting");
    setTargetX(null);
    setTargetY(null);
    pendingRef.current = false;

    timerRef.current = setTimeout(() => {
      const x = 0.1 + Math.random() * 0.8;
      const y = 0.1 + Math.random() * 0.8;
      setTargetX(x);
      setTargetY(y);
      displayedAtRef.current = performance.now();
      pendingRef.current = true;
      setStatus("ready");
    }, delay);
  }, []);

  const start = useCallback((trials = 10) => {
    setTotal(trials);
    setResult(null);
    trialsRef.current = [];
    idxRef.current = 0;
    setCurrent(0);
    showNext();
  }, [showNext]);

  const recordTap = useCallback((tapX: number, tapY: number, containerW: number, containerH: number) => {
    if (!pendingRef.current || targetX === null || targetY === null) return;
    pendingRef.current = false;
    if (timerRef.current !== null) clearTimeout(timerRef.current);

    const now = performance.now();
    const reactionMs = now - displayedAtRef.current;
    const cx = targetX * containerW;
    const cy = targetY * containerH;
    const dist = Math.sqrt((tapX - cx) ** 2 + (tapY - cy) ** 2);
    const hit = dist <= TARGET_RADIUS_PX * 2; // generous for small screens

    trialsRef.current.push({
      targetX,
      targetY,
      displayedAt: displayedAtRef.current,
      respondedAt: now,
      reactionMs,
      hitDistance: Math.round(dist),
      hit,
    });

    idxRef.current++;
    setCurrent(idxRef.current);

    if (idxRef.current < totalTrials) {
      showNext();
    } else {
      const rts = trialsRef.current.filter(t => t.reactionMs !== null).map(t => t.reactionMs!);
      const missCount = trialsRef.current.filter(t => !t.hit).length;
      const meanMs = rts.length > 0 ? rts.reduce((a, b) => a + b, 0) / rts.length : 0;
      const screen: ReactionResult["cognitiveScreen"] =
        meanMs < NORMAL_MEAN_MS ? "normal" : meanMs < BORDERLINE_MEAN_MS ? "borderline" : "impaired";

      setResult({
        meanReactionMs: Math.round(meanMs),
        medianReactionMs: Math.round(median(rts)),
        cvPercent: Math.round(cv(rts)),
        missRate: Math.round((missCount / trialsRef.current.length) * 100) / 100,
        cognitiveScreen: screen,
        trials: [...trialsRef.current],
        measuredAt: new Date().toISOString(),
      });
      setTargetX(null);
      setTargetY(null);
      setStatus("done");
    }
  }, [targetX, targetY, totalTrials, showNext]);

  const toObservation = useCallback(
    (r: ReactionResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "72172-0", display: "Reaction time" }], text: "Simple reaction time" },
      effectiveDateTime: r.measuredAt,
      valueQuantity: { value: r.meanReactionMs, unit: "ms", system: "http://unitsofmeasure.org", code: "ms" },
      component: [
        { code: { text: "Median reaction time" }, valueQuantity: { value: r.medianReactionMs, unit: "ms" } },
        { code: { text: "Coefficient of variation" }, valueQuantity: { value: r.cvPercent, unit: "%" } },
        { code: { text: "Miss rate" }, valueQuantity: { value: r.missRate, unit: "ratio" } },
        { code: { text: "Cognitive screen" }, valueString: r.cognitiveScreen },
      ],
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, currentTrial, totalTrials, targetX, targetY, result, start, recordTap, toObservation };
}
