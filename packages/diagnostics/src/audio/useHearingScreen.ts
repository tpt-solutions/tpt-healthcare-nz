import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Pure-tone audiometry screening via Web Audio API.
// Presents tones at standard audiometric frequencies and step-down levels.
// Uses a simplified Hughson-Westlake threshold procedure:
//   - Start at 40 dBHL
//   - Descend 10 dB after a correct response, ascend 5 dB after a miss
//   - Threshold = lowest level with ≥2 correct responses out of ≤3 presentations
//
// IMPORTANT: Output level is uncalibrated — true audiometric calibration
// requires a known headphone transducer. Use only for screening / referral
// decisions, not definitive threshold measurement.

export type HearingStatus = "idle" | "testing" | "done" | "error";

export interface HearingTrial {
  freqHz: number;
  ear: "left" | "right";
  levelDbHL: number;       // presentation level (nominal, uncalibrated)
  heard: boolean;
}

export interface EarThreshold {
  freqHz: number;
  thresholdDbHL: number;
  category: "normal" | "mild" | "moderate" | "severe" | "profound";
}

export interface HearingResult {
  leftEar: EarThreshold[];
  rightEar: EarThreshold[];
  overallClassification: "normal" | "mild-loss" | "moderate-loss" | "severe-loss";
  trials: HearingTrial[];
  measuredAt: string;
}

export interface UseHearingScreenReturn {
  status: HearingStatus;
  error: string | null;
  currentFreqHz: number | null;
  currentEar: "left" | "right" | null;
  currentLevelDbHL: number | null;
  result: HearingResult | null;
  start: () => void;
  recordResponse: (heard: boolean) => void;
  toObservation: (r: HearingResult, patientId?: string, encounterId?: string) => Observation;
}

// Screening frequencies (Hz)
const SCREEN_FREQS = [500, 1000, 2000, 4000, 8000];
const EARS: Array<"left" | "right"> = ["right", "left"];

// Nominal dBHL→gain mapping (linear amplitude, calibration-free approximation)
// 0 dBHL ≈ 1e-5 Pa; practical web audio full-scale ≈ 90 dBSPL with average
// headphones. We map dBHL to a normalised gain assuming 60 dBHL = gain 1.0.
function dbHLToGain(dbHL: number): number {
  return Math.pow(10, (dbHL - 60) / 20);
}

function thresholdCategory(dbHL: number): EarThreshold["category"] {
  if (dbHL <= 25)  return "normal";
  if (dbHL <= 40)  return "mild";
  if (dbHL <= 55)  return "moderate";
  if (dbHL <= 70)  return "severe";
  return "profound";
}

interface FreqState {
  level: number;
  correctAtLevel: Map<number, number>;
  trialsAtLevel: Map<number, number>;
  threshold: number | null;
}

export function useHearingScreen(): UseHearingScreenReturn {
  const [status, setStatus]   = useState<HearingStatus>("idle");
  const [error, setError]     = useState<string | null>(null);
  const [currentFreqHz, setFreq] = useState<number | null>(null);
  const [currentEar, setEar]  = useState<"left" | "right" | null>(null);
  const [currentLevel, setLevel] = useState<number | null>(null);
  const [result, setResult]   = useState<HearingResult | null>(null);

  const ctxRef    = useRef<AudioContext | null>(null);
  const trialsRef = useRef<HearingTrial[]>([]);

  // Per-freq, per-ear state for Hughson-Westlake
  const stateRef  = useRef<Map<string, FreqState>>(new Map());
  const queueRef  = useRef<Array<{ freq: number; ear: "left" | "right" }>>([]);
  const currentRef = useRef<{ freq: number; ear: "left" | "right" } | null>(null);

  const buildQueue = useCallback(() => {
    const q: Array<{ freq: number; ear: "left" | "right" }> = [];
    for (const ear of EARS) for (const freq of SCREEN_FREQS) q.push({ freq, ear });
    return q;
  }, []);

  const initState = useCallback(() => {
    const map = new Map<string, FreqState>();
    for (const ear of EARS) {
      for (const freq of SCREEN_FREQS) {
        map.set(`${ear}-${freq}`, {
          level: 40,
          correctAtLevel: new Map(),
          trialsAtLevel: new Map(),
          threshold: null,
        });
      }
    }
    return map;
  }, []);

  const playTone = useCallback((freqHz: number, ear: "left" | "right", dbHL: number) => {
    const ctx = ctxRef.current;
    if (!ctx) return;
    const gain = ctx.createGain();
    const panner = ctx.createStereoPanner();
    const osc = ctx.createOscillator();

    osc.type = "sine";
    osc.frequency.value = freqHz;
    gain.gain.value = Math.max(0, Math.min(1, dbHLToGain(dbHL)));
    panner.pan.value = ear === "left" ? -1 : 1;

    osc.connect(gain);
    gain.connect(panner);
    panner.connect(ctx.destination);

    const now = ctx.currentTime;
    // 200 ms ramp on/off to avoid clicks
    gain.gain.setValueAtTime(0, now);
    gain.gain.linearRampToValueAtTime(gain.gain.value, now + 0.2);
    osc.start(now);
    osc.stop(now + 1.0); // 1 s tone
    gain.gain.linearRampToValueAtTime(0, now + 1.0);
  }, []);

  const advance = useCallback(() => {
    const q = queueRef.current;
    if (q.length === 0) {
      // All done — compile results
      const compile = (ear: "left" | "right"): EarThreshold[] =>
        SCREEN_FREQS.map(freq => {
          const s = stateRef.current.get(`${ear}-${freq}`)!;
          const th = s.threshold ?? s.level;
          return { freqHz: freq, thresholdDbHL: th, category: thresholdCategory(th) };
        });

      const left  = compile("left");
      const right = compile("right");
      const worst = Math.max(...[...left, ...right].map(t => t.thresholdDbHL));
      const overall: HearingResult["overallClassification"] =
        worst <= 25 ? "normal" : worst <= 40 ? "mild-loss" : worst <= 55 ? "moderate-loss" : "severe-loss";

      setResult({ leftEar: left, rightEar: right, overallClassification: overall, trials: [...trialsRef.current], measuredAt: new Date().toISOString() });
      setFreq(null); setEar(null); setLevel(null);
      setStatus("done");
      return;
    }

    const next = q[0];
    currentRef.current = next;
    const s = stateRef.current.get(`${next.ear}-${next.freq}`)!;
    setFreq(next.freq);
    setEar(next.ear);
    setLevel(s.level);
    playTone(next.freq, next.ear, s.level);
  }, [playTone]);

  const start = useCallback(() => {
    setError(null);
    setResult(null);
    trialsRef.current = [];
    stateRef.current  = initState();
    queueRef.current  = buildQueue();

    try {
      ctxRef.current = new AudioContext();
    } catch {
      setError("Web Audio API not available");
      setStatus("error");
      return;
    }
    setStatus("testing");
    advance();
  }, [advance, buildQueue, initState]);

  const recordResponse = useCallback((heard: boolean) => {
    const cur = currentRef.current;
    if (!cur) return;

    const key = `${cur.ear}-${cur.freq}`;
    const s   = stateRef.current.get(key)!;

    trialsRef.current.push({ freqHz: cur.freq, ear: cur.ear, levelDbHL: s.level, heard });

    const prev = s.correctAtLevel.get(s.level) ?? 0;
    const prevT = s.trialsAtLevel.get(s.level) ?? 0;
    s.correctAtLevel.set(s.level, prev + (heard ? 1 : 0));
    s.trialsAtLevel.set(s.level, prevT + 1);

    const correct = s.correctAtLevel.get(s.level)!;
    const total   = s.trialsAtLevel.get(s.level)!;

    if (correct >= 2) {
      // Threshold found
      s.threshold = s.level;
      queueRef.current.shift(); // move to next freq/ear
    } else if (total - correct >= 2) {
      // Two misses at this level — go up 5 dB
      s.level = Math.min(100, s.level + 5);
    } else if (heard) {
      // Descend 10 dB after a correct response
      s.level = Math.max(0, s.level - 10);
    } else {
      // One miss — ascend 5 dB
      s.level = Math.min(100, s.level + 5);
    }

    setLevel(s.level);
    advance();
  }, [advance]);

  const toObservation = useCallback(
    (r: HearingResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "89016-0", display: "Hearing threshold" }], text: "Pure-tone hearing screening" },
      effectiveDateTime: r.measuredAt,
      valueString: r.overallClassification,
      component: [
        ...r.rightEar.map(t => ({
          code: { text: `Right ear threshold ${t.freqHz} Hz` },
          valueQuantity: { value: t.thresholdDbHL, unit: "dB HL" },
        })),
        ...r.leftEar.map(t => ({
          code: { text: `Left ear threshold ${t.freqHz} Hz` },
          valueQuantity: { value: t.thresholdDbHL, unit: "dB HL" },
        })),
      ],
      method: { text: "Smartphone pure-tone screening — uncalibrated" },
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, currentFreqHz, currentEar, currentLevelDbHL: currentLevel, result, start, recordResponse, toObservation };
}
