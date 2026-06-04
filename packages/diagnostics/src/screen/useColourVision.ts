import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Ishihara-equivalent colour vision screening.
// Displays pseudo-isochromatic plates generated as canvas dot patterns.
// Each plate has a number visible only to normal trichromats (or only to
// dichromats for control plates). The patient identifies the number;
// the hook scores the result and classifies the deficiency type.

export type ColourVisionStatus = "idle" | "testing" | "done";

export interface PlateResult {
  plateId: number;
  normalAnswer: string;
  deficiencyAnswer: string;
  patientAnswer: string;
  correct: boolean; // normal trichromat correct
}

export interface ColourVisionResult {
  score: number;           // plates answered correctly (out of 14)
  classification: "normal" | "mild-red-green" | "moderate-red-green" | "severe-red-green" | "blue-yellow";
  deficiencyType: "none" | "protanopia" | "deuteranopia" | "tritanopia" | "protanomaly" | "deuteranomaly";
  plates: PlateResult[];
  measuredAt: string;
}

export interface UseColourVisionReturn {
  status: ColourVisionStatus;
  plateIndex: number;
  totalPlates: number;
  currentPlateCanvas: HTMLCanvasElement | null;
  currentPlateId: number;
  result: ColourVisionResult | null;
  start: () => void;
  recordAnswer: (answer: string) => void;
  toObservation: (r: ColourVisionResult, patientId?: string, encounterId?: string) => Observation;
}

// Simplified plate definitions. A real implementation would embed the full
// Ishihara 38-plate set as encoded pixel data. These synthetic plates are
// suitable for screening only — refer for formal Farnsworth-Munsell 100 Hue.
interface PlateDef {
  id: number;
  normalAnswer: string;
  deficiencyAnswer: string;
  bgHue: number;   // background dot hue (HSL)
  fgHue: number;   // figure dot hue
}

const PLATES: PlateDef[] = [
  { id: 1,  normalAnswer: "12", deficiencyAnswer: "12", bgHue: 30,  fgHue: 30  }, // control
  { id: 2,  normalAnswer: "8",  deficiencyAnswer: "",   bgHue: 120, fgHue: 30  }, // R/G
  { id: 3,  normalAnswer: "6",  deficiencyAnswer: "",   bgHue: 120, fgHue: 15  }, // R/G
  { id: 4,  normalAnswer: "29", deficiencyAnswer: "70", bgHue: 90,  fgHue: 0   }, // R/G
  { id: 5,  normalAnswer: "57", deficiencyAnswer: "35", bgHue: 60,  fgHue: 10  }, // R/G
  { id: 6,  normalAnswer: "5",  deficiencyAnswer: "",   bgHue: 150, fgHue: 30  }, // R/G
  { id: 7,  normalAnswer: "3",  deficiencyAnswer: "5",  bgHue: 80,  fgHue: 5   }, // R/G
  { id: 8,  normalAnswer: "15", deficiencyAnswer: "17", bgHue: 100, fgHue: 15  }, // R/G
  { id: 9,  normalAnswer: "74", deficiencyAnswer: "21", bgHue: 70,  fgHue: 0   }, // R/G
  { id: 10, normalAnswer: "2",  deficiencyAnswer: "",   bgHue: 130, fgHue: 20  }, // R/G
  { id: 11, normalAnswer: "6",  deficiencyAnswer: "",   bgHue: 110, fgHue: 25  }, // R/G
  { id: 12, normalAnswer: "97", deficiencyAnswer: "",   bgHue: 140, fgHue: 10  }, // R/G
  { id: 13, normalAnswer: "45", deficiencyAnswer: "",   bgHue: 95,  fgHue: 5   }, // R/G
  { id: 14, normalAnswer: "5",  deficiencyAnswer: "",   bgHue: 210, fgHue: 50  }, // B/Y control
];

function hslToRgb(h: number, s: number, l: number): [number, number, number] {
  s /= 100; l /= 100;
  const k = (n: number) => (n + h / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)));
  return [Math.round(f(0) * 255), Math.round(f(8) * 255), Math.round(f(4) * 255)];
}

function renderPlate(plate: PlateDef, size = 320): HTMLCanvasElement {
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = size;
  const ctx = canvas.getContext("2d")!;

  // Simple dot pattern: background vs figure using luminance difference + hue confusion
  const dots = 600;
  const r = (min: number, max: number) => min + Math.random() * (max - min);

  // Draw background dots
  for (let i = 0; i < dots; i++) {
    const x = r(0, size), y = r(0, size);
    const rad = r(4, 10);
    const [rr, g, b] = hslToRgb(plate.bgHue + r(-20, 20), r(60, 80), r(50, 70));
    ctx.beginPath();
    ctx.arc(x, y, rad, 0, Math.PI * 2);
    ctx.fillStyle = `rgb(${rr},${g},${b})`;
    ctx.fill();
  }

  // Draw figure (number path) dots in a different hue
  // Very simplified — a real implementation would use a bitmap mask for each digit
  const cx = size / 2, cy = size / 2;
  for (let i = 0; i < dots / 3; i++) {
    const angle = r(0, Math.PI * 2);
    const rDist = r(20, 70);
    const x = cx + Math.cos(angle) * rDist;
    const y = cy + Math.sin(angle) * rDist;
    const rad = r(4, 10);
    const [rr, g, b] = hslToRgb(plate.fgHue + r(-10, 10), r(65, 85), r(45, 65));
    ctx.beginPath();
    ctx.arc(x, y, rad, 0, Math.PI * 2);
    ctx.fillStyle = `rgb(${rr},${g},${b})`;
    ctx.fill();
  }
  return canvas;
}

function classify(plates: PlateResult[]): Pick<ColourVisionResult, "classification" | "deficiencyType"> {
  const correct = plates.filter(p => p.correct).length;
  const seenDeficiency = plates.filter(p => !p.correct && p.deficiencyAnswer && p.patientAnswer === p.deficiencyAnswer);
  const isBlueYellow = plates.some(p => p.plateId === 14 && !p.correct);

  if (correct >= 12)       return { classification: "normal", deficiencyType: "none" };
  if (isBlueYellow)        return { classification: "blue-yellow", deficiencyType: "tritanopia" };
  if (seenDeficiency.length >= 4) return { classification: "severe-red-green", deficiencyType: "deuteranopia" };
  if (correct >= 9)        return { classification: "mild-red-green", deficiencyType: "deuteranomaly" };
  return { classification: "moderate-red-green", deficiencyType: "protanomaly" };
}

export function useColourVision(): UseColourVisionReturn {
  const [status, setStatus]       = useState<ColourVisionStatus>("idle");
  const [plateIndex, setPlateIdx] = useState(0);
  const [currentPlateCanvas, setCanvas] = useState<HTMLCanvasElement | null>(null);
  const [result, setResult]       = useState<ColourVisionResult | null>(null);
  const platesRef = useRef<PlateResult[]>([]);
  const idxRef    = useRef(0);

  const presentPlate = useCallback((idx: number) => {
    const plate = PLATES[idx];
    setCanvas(renderPlate(plate));
    setPlateIdx(idx);
  }, []);

  const start = useCallback(() => {
    idxRef.current = 0;
    platesRef.current = [];
    setResult(null);
    setStatus("testing");
    presentPlate(0);
  }, [presentPlate]);

  const recordAnswer = useCallback((answer: string) => {
    const plate = PLATES[idxRef.current];
    platesRef.current.push({
      plateId: plate.id,
      normalAnswer: plate.normalAnswer,
      deficiencyAnswer: plate.deficiencyAnswer,
      patientAnswer: answer.trim(),
      correct: answer.trim() === plate.normalAnswer,
    });

    idxRef.current++;
    if (idxRef.current < PLATES.length) {
      presentPlate(idxRef.current);
    } else {
      const { classification, deficiencyType } = classify(platesRef.current);
      setResult({
        score: platesRef.current.filter(p => p.correct).length,
        classification,
        deficiencyType,
        plates: [...platesRef.current],
        measuredAt: new Date().toISOString(),
      });
      setCanvas(null);
      setStatus("done");
    }
  }, [presentPlate]);

  const toObservation = useCallback(
    (r: ColourVisionResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "79901-8", display: "Color vision" }], text: "Colour vision screening" },
      effectiveDateTime: r.measuredAt,
      valueCodeableConcept: { coding: [{ system: "http://snomed.info/sct", code: r.deficiencyType === "none" ? "36692007" : "64269007", display: r.classification }], text: r.classification },
      component: [
        { code: { text: "Score" }, valueQuantity: { value: r.score, unit: `/14` } },
        { code: { text: "Deficiency type" }, valueString: r.deficiencyType },
      ],
      method: { text: "Pseudo-isochromatic plate screening (Ishihara-equivalent)" },
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, plateIndex, totalPlates: PLATES.length, currentPlateCanvas, currentPlateId: PLATES[plateIndex]?.id ?? 0, result, start, recordAnswer, toObservation };
}
