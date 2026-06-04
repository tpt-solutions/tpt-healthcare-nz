import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

export type RespirationStatus = "idle" | "requesting" | "measuring" | "done" | "error";

export interface RespirationResult {
  breathsPerMin: number;
  measuredAt: string;
}

export interface UseRespirationRateReturn {
  status: RespirationStatus;
  error: string | null;
  breathsPerMin: number | null;
  progress: number;
  videoRef: React.RefObject<HTMLVideoElement>;
  start: () => Promise<void>;
  stop: () => void;
  toObservation: (result: RespirationResult, patientId?: string, encounterId?: string) => Observation;
}

const SAMPLE_DURATION_MS = 30_000; // 30 s for a reliable respiratory rate
const FPS = 15;
const FRAME_INTERVAL_MS = 1000 / FPS;
const RR_MIN_HZ = 0.1;  // 6 breaths/min
const RR_MAX_HZ = 0.67; // 40 breaths/min

function dominantFreq(samples: number[], fps: number, minHz: number, maxHz: number): number {
  const n = samples.length;
  const mean = samples.reduce((a, b) => a + b, 0) / n;
  const centered = samples.map(v => v - mean);
  let bestFreq = 0, bestAmp = 0;
  for (let k = 1; k < n / 2; k++) {
    const freq = (k * fps) / n;
    if (freq < minHz || freq > maxHz) continue;
    let re = 0, im = 0;
    for (let i = 0; i < n; i++) {
      const angle = (2 * Math.PI * k * i) / n;
      re += centered[i] * Math.cos(angle);
      im -= centered[i] * Math.sin(angle);
    }
    const amp = Math.sqrt(re * re + im * im);
    if (amp > bestAmp) { bestAmp = amp; bestFreq = freq; }
  }
  return bestFreq;
}

// Track overall brightness change in the centre strip of each frame as a
// chest-rise proxy (works best with a stationary patient facing the camera).
function frameBrightness(imageData: ImageData, roiY: number, roiH: number): number {
  const { data, width } = imageData;
  let sum = 0, count = 0;
  const roiX = Math.floor(width * 0.25);
  const roiW = Math.floor(width * 0.5);
  for (let y = roiY; y < roiY + roiH; y++) {
    for (let x = roiX; x < roiX + roiW; x++) {
      const idx = (y * width + x) * 4;
      sum += 0.299 * data[idx] + 0.587 * data[idx + 1] + 0.114 * data[idx + 2];
      count++;
    }
  }
  return count > 0 ? sum / count : 0;
}

export function useRespirationRate(): UseRespirationRateReturn {
  const [status, setStatus] = useState<RespirationStatus>("idle");
  const [error, setError] = useState<string | null>(null);
  const [breathsPerMin, setBreathsPerMin] = useState<number | null>(null);
  const [progress, setProgress] = useState(0);
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const rafRef = useRef<number | null>(null);
  const samplesRef = useRef<number[]>([]);
  const startTimeRef = useRef<number>(0);
  const lastFrameRef = useRef<number>(0);

  const stop = useCallback(() => {
    if (rafRef.current !== null) cancelAnimationFrame(rafRef.current);
    streamRef.current?.getTracks().forEach(t => t.stop());
    streamRef.current = null;
    setStatus("idle");
    setProgress(0);
  }, []);

  const start = useCallback(async () => {
    setError(null);
    setBreathsPerMin(null);
    setProgress(0);
    setStatus("requesting");
    samplesRef.current = [];

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "user", width: { ideal: 640 }, height: { ideal: 480 } },
        audio: false,
      });
      streamRef.current = stream;
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
        await videoRef.current.play();
      }

      const canvas = document.createElement("canvas");
      const video = videoRef.current!;
      canvas.width = video.videoWidth || 640;
      canvas.height = video.videoHeight || 480;
      const ctx = canvas.getContext("2d", { willReadFrequently: true })!;
      const roiH = Math.floor(canvas.height * 0.4);
      const roiY = Math.floor(canvas.height * 0.3);

      setStatus("measuring");
      startTimeRef.current = performance.now();
      lastFrameRef.current = 0;

      const tick = (now: number) => {
        if (!streamRef.current) return;
        const elapsed = now - startTimeRef.current;
        setProgress(Math.min(100, Math.round((elapsed / SAMPLE_DURATION_MS) * 100)));

        if (now - lastFrameRef.current >= FRAME_INTERVAL_MS) {
          lastFrameRef.current = now;
          ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
          const frame = ctx.getImageData(0, 0, canvas.width, canvas.height);
          samplesRef.current.push(frameBrightness(frame, roiY, roiH));
        }

        if (elapsed < SAMPLE_DURATION_MS) {
          rafRef.current = requestAnimationFrame(tick);
        } else {
          const freq = dominantFreq(samplesRef.current, FPS, RR_MIN_HZ, RR_MAX_HZ);
          streamRef.current?.getTracks().forEach(t => t.stop());
          streamRef.current = null;
          setBreathsPerMin(Math.round(freq * 60));
          setProgress(100);
          setStatus("done");
        }
      };
      rafRef.current = requestAnimationFrame(tick);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Camera unavailable");
      setStatus("error");
    }
  }, []);

  const toObservation = useCallback(
    (result: RespirationResult, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "vital-signs" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "9279-1", display: "Respiratory rate" }], text: "Respiratory rate" },
      effectiveDateTime: result.measuredAt,
      valueQuantity: { value: result.breathsPerMin, unit: "/min", system: "http://unitsofmeasure.org", code: "/min" },
      method: { text: "Camera-based chest motion analysis" },
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, breathsPerMin, progress, videoRef, start, stop, toObservation };
}
