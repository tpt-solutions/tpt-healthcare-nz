import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Snellen-equivalent visual acuity test.
// Displays optotypes (letters) at sizes corresponding to 20/200 → 20/20
// at a calibrated 40 cm viewing distance (arm's length from phone screen).
// The patient identifies each letter; clinician records correct/incorrect.
// Physical pixel size is estimated from screen.availWidth / devicePixelRatio
// and a nominal 160 dpi baseline — accurate enough for screening.

export type AcuityStatus = "idle" | "testing" | "done";

export interface AcuityTrial {
  snellenLine: string;     // e.g. "20/40"
  logMAR: number;          // e.g. 0.3
  optotype: string;        // letter shown
  correct: boolean;
}

export interface AcuityResult {
  eye: "left" | "right" | "both";
  snellen: string;         // best line where ≥3/5 correct
  logMAR: number;
  trials: AcuityTrial[];
  measuredAt: string;
}

export interface UseVisualAcuityReturn {
  status: AcuityStatus;
  currentOptotype: string | null;
  currentSizePx: number;
  lineIndex: number;
  totalLines: number;
  result: AcuityResult | null;
  start: (eye?: AcuityResult["eye"]) => void;
  recordResponse: (correct: boolean) => void;
  toObservation: (r: AcuityResult, patientId?: string, encounterId?: string) => Observation;
}

// Standard Snellen chart rows (worst → best)
const SNELLEN_LINES: Array<{ label: string; logMAR: number; letters: string[] }> = [
  { label: "20/200", logMAR: 1.0, letters: ["E"] },
  { label: "20/160", logMAR: 0.9, letters: ["F", "P"] },
  { label: "20/125", logMAR: 0.8, letters: ["T", "O", "Z"] },
  { label: "20/100", logMAR: 0.7, letters: ["L", "P", "E", "D"] },
  { label: "20/80",  logMAR: 0.6, letters: ["P", "E", "C", "F", "D"] },
  { label: "20/63",  logMAR: 0.5, letters: ["E", "D", "F", "C", "Z", "P"] },
  { label: "20/50",  logMAR: 0.4, letters: ["F", "E", "L", "O", "P", "Z", "D"] },
  { label: "20/40",  logMAR: 0.3, letters: ["D", "E", "F", "P", "O", "T", "E", "C"] },
  { label: "20/32",  logMAR: 0.2, letters: ["L", "E", "F", "O", "D", "P", "C", "T"] },
  { label: "20/25",  logMAR: 0.1, letters: ["F", "D", "P", "L", "T", "C", "E", "O"] },
  { label: "20/20",  logMAR: 0.0, letters: ["E", "F", "P", "T", "O", "L", "Z", "D"] },
];

const VIEWING_DISTANCE_CM = 40;
const NOMINAL_DPI = 160;

// Snellen letter height in mm for a given logMAR at 40 cm:
// heightMM = 2 * viewingDistanceMM * tan(5' × logMAR_multiplier / 2)
// Simplified: at 6 m, 1 arcmin ≈ 1.745 mm; at 40 cm scale proportionally.
function optotypeSizePx(logMAR: number): number {
  const arcminHeight = 5 * Math.pow(10, logMAR); // arcminutes for that line
  const heightMM = 2 * (VIEWING_DISTANCE_CM * 10) * Math.tan((arcminHeight * Math.PI) / (60 * 180));
  const pxPerMM = (NOMINAL_DPI / 25.4) * (window.devicePixelRatio || 1);
  return Math.round(heightMM * pxPerMM);
}

export function useVisualAcuity(): UseVisualAcuityReturn {
  const [status, setStatus]           = useState<AcuityStatus>("idle");
  const [currentOptotype, setOptotype]= useState<string | null>(null);
  const [currentSizePx, setSizePx]    = useState(0);
  const [lineIndex, setLineIndex]     = useState(0);
  const [result, setResult]           = useState<AcuityResult | null>(null);

  const trialsRef   = useRef<AcuityTrial[]>([]);
  const eyeRef      = useRef<AcuityResult["eye"]>("both");
  const lineRef     = useRef(0);
  const lineCorrectRef = useRef(0);
  const lineLetterIdxRef = useRef(0);
  const LETTERS_PER_LINE = 5;

  const presentLetter = useCallback((idx: number) => {
    const line = SNELLEN_LINES[idx];
    const letter = line.letters[Math.floor(Math.random() * line.letters.length)];
    setOptotype(letter);
    setSizePx(optotypeSizePx(line.logMAR));
    lineLetterIdxRef.current = 0;
    lineCorrectRef.current   = 0;
  }, []);

  const start = useCallback((eye: AcuityResult["eye"] = "both") => {
    eyeRef.current = eye;
    trialsRef.current = [];
    lineRef.current = 0;
    setLineIndex(0);
    setResult(null);
    setStatus("testing");
    presentLetter(0);
  }, [presentLetter]);

  const recordResponse = useCallback((correct: boolean) => {
    const idx  = lineRef.current;
    const line = SNELLEN_LINES[idx];
    const letter = currentOptotype ?? "?";

    trialsRef.current.push({ snellenLine: line.label, logMAR: line.logMAR, optotype: letter, correct });
    if (correct) lineCorrectRef.current++;
    lineLetterIdxRef.current++;

    if (lineLetterIdxRef.current >= LETTERS_PER_LINE) {
      const passed = lineCorrectRef.current >= 3;
      if (passed && idx < SNELLEN_LINES.length - 1) {
        lineRef.current++;
        setLineIndex(lineRef.current);
        presentLetter(lineRef.current);
      } else {
        // Best passing line is current if passed, else one above
        const bestIdx = passed ? idx : Math.max(0, idx - 1);
        const best = SNELLEN_LINES[bestIdx];
        setResult({
          eye: eyeRef.current,
          snellen: best.label,
          logMAR: best.logMAR,
          trials: [...trialsRef.current],
          measuredAt: new Date().toISOString(),
        });
        setOptotype(null);
        setStatus("done");
      }
    } else {
      const nextLetter = line.letters[Math.floor(Math.random() * line.letters.length)];
      setOptotype(nextLetter);
    }
  }, [currentOptotype, presentLetter]);

  const toObservation = useCallback(
    (r: AcuityResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "79881-2", display: "Visual acuity" }], text: "Visual acuity — Snellen" },
      effectiveDateTime: r.measuredAt,
      valueString: r.snellen,
      component: [
        { code: { text: "logMAR" }, valueQuantity: { value: r.logMAR, unit: "logMAR" } },
        { code: { text: "Eye tested" }, valueString: r.eye },
      ],
      method: { text: "Smartphone Snellen chart — 40 cm" },
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return {
    status, currentOptotype, currentSizePx, lineIndex,
    totalLines: SNELLEN_LINES.length, result, start, recordResponse, toObservation,
  };
}
