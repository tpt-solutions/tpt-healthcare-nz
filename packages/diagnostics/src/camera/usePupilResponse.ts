import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

// Pupil light reflex assessment — flash bright screen, record pupil constriction.
// The user holds the device close while a clinician or automated analysis measures
// the change in pupil radius. Used as a bedside neurological screening tool.

export type PupilStatus = "idle" | "requesting" | "baseline" | "flash" | "recording" | "done" | "error";

export interface PupilMeasurement {
  eye: "left" | "right";
  baselineRadiusPx: number;
  minRadiusPx: number;
  constrictionPercent: number;
  latencyMs: number; // time from flash onset to min-radius
  measuredAt: string;
}

export interface UsePupilResponseReturn {
  status: PupilStatus;
  error: string | null;
  result: PupilMeasurement | null;
  videoRef: React.RefObject<HTMLVideoElement>;
  start: (eye?: "left" | "right") => Promise<void>;
  stop: () => void;
  toObservation: (m: PupilMeasurement, patientId?: string, encounterId?: string) => Observation;
}

const BASELINE_MS = 2_000;   // record baseline before flash
const RECORD_MS  = 3_000;   // record after flash
const FLASH_MS   = 500;      // flash duration
const FPS        = 30;
const FRAME_MS   = 1000 / FPS;

// Rough pupil radius via dark-pixel count in a central ROI (iris is darker than sclera).
// This is a coarse proxy; production would use a proper circle-Hough or ML model.
function estimatePupilRadius(imageData: ImageData, threshold = 60): number {
  const { data, width, height } = imageData;
  const cx = width / 2, cy = height / 2;
  const roi = Math.min(width, height) * 0.3;
  let dark = 0, total = 0;
  for (let y = Math.floor(cy - roi); y < cy + roi; y++) {
    for (let x = Math.floor(cx - roi); x < cx + roi; x++) {
      const dx = x - cx, dy = y - cy;
      if (dx * dx + dy * dy > roi * roi) continue;
      const idx = (y * width + x) * 4;
      const lum = 0.299 * data[idx] + 0.587 * data[idx + 1] + 0.114 * data[idx + 2];
      if (lum < threshold) dark++;
      total++;
    }
  }
  return total > 0 ? Math.sqrt((dark / total) * roi * roi * Math.PI) : 0;
}

export function usePupilResponse(): UsePupilResponseReturn {
  const [status, setStatus] = useState<PupilStatus>("idle");
  const [error, setError]   = useState<string | null>(null);
  const [result, setResult] = useState<PupilMeasurement | null>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const rafRef    = useRef<number | null>(null);
  const eyeRef    = useRef<"left" | "right">("left");

  const stop = useCallback(() => {
    if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    streamRef.current?.getTracks().forEach(t => t.stop());
    streamRef.current = null;
    setStatus("idle");
  }, []);

  const start = useCallback(async (eye: "left" | "right" = "left") => {
    eyeRef.current = eye;
    setError(null);
    setResult(null);
    setStatus("requesting");

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "user", width: { ideal: 640 }, height: { ideal: 480 } },
        audio: false,
      });
      streamRef.current = stream;
      const video = videoRef.current!;
      video.srcObject = stream;
      await video.play();

      const canvas = document.createElement("canvas");
      canvas.width = video.videoWidth || 640;
      canvas.height = video.videoHeight || 480;
      const ctx = canvas.getContext("2d", { willReadFrequently: true })!;

      const baselineRadii: number[] = [];
      const postFlashRadii: number[] = [];
      const postFlashTimestamps: number[] = [];
      let phase: "baseline" | "flash" | "record" = "baseline";
      const phaseStart = performance.now();

      setStatus("baseline");
      let lastFrame = 0;

      const tick = (now: number) => {
        if (!streamRef.current) return;
        const elapsed = now - phaseStart;

        if (now - lastFrame >= FRAME_MS) {
          lastFrame = now;
          ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
          const frame = ctx.getImageData(0, 0, canvas.width, canvas.height);
          const r = estimatePupilRadius(frame);

          if (phase === "baseline") {
            baselineRadii.push(r);
            if (elapsed >= BASELINE_MS) {
              phase = "flash";
              setStatus("flash");
            }
          } else if (phase === "record") {
            postFlashRadii.push(r);
            postFlashTimestamps.push(now);
            if (elapsed - BASELINE_MS - FLASH_MS >= RECORD_MS) {
              // Analyse
              const baseline = baselineRadii.reduce((a, b) => a + b, 0) / baselineRadii.length;
              const minR = Math.min(...postFlashRadii);
              const minIdx = postFlashRadii.indexOf(minR);
              const latency = minIdx >= 0 ? postFlashTimestamps[minIdx] - (phaseStart + BASELINE_MS + FLASH_MS) : 0;
              streamRef.current?.getTracks().forEach(t => t.stop());
              streamRef.current = null;
              setResult({
                eye: eyeRef.current,
                baselineRadiusPx: baseline,
                minRadiusPx: minR,
                constrictionPercent: baseline > 0 ? Math.round(((baseline - minR) / baseline) * 100) : 0,
                latencyMs: Math.max(0, Math.round(latency)),
                measuredAt: new Date().toISOString(),
              });
              setStatus("done");
              return;
            }
          }
        }

        // Transition flash → record after FLASH_MS
        if (phase === "flash" && now - phaseStart - BASELINE_MS >= FLASH_MS) {
          phase = "record";
          setStatus("recording");
        }

        rafRef.current = requestAnimationFrame(tick);
      };
      rafRef.current = requestAnimationFrame(tick);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Camera unavailable");
      setStatus("error");
    }
  }, []);

  const toObservation = useCallback(
    (m: PupilMeasurement, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "exam" }] }],
      code: { coding: [{ system: "http://snomed.info/sct", code: "363953009", display: "Pupil light reflex" }], text: "Pupil light reflex" },
      effectiveDateTime: m.measuredAt,
      bodySite: { coding: [{ system: "http://snomed.info/sct", code: m.eye === "left" ? "8966001" : "18944008", display: `${m.eye} eye` }] },
      component: [
        { code: { text: "Constriction %" }, valueQuantity: { value: m.constrictionPercent, unit: "%" } },
        { code: { text: "Latency ms" }, valueQuantity: { value: m.latencyMs, unit: "ms" } },
        { code: { text: "Baseline pupil radius px" }, valueQuantity: { value: m.baselineRadiusPx, unit: "px" } },
      ],
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, result, videoRef, start, stop, toObservation };
}
