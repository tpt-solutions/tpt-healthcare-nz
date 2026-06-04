import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

export type HeartRateStatus = "idle" | "requesting" | "acquiring" | "measuring" | "done" | "error";

export interface HeartRateSample {
  bpm: number;
  confidence: number; // 0–1, signal quality estimate
  measuredAt: string;
}

export interface UseHeartRateReturn {
  status: HeartRateStatus;
  error: string | null;
  bpm: number | null;
  confidence: number | null;
  progress: number; // 0–100 during measurement
  start: () => Promise<void>;
  stop: () => void;
  toObservation: (sample: HeartRateSample, patientId?: string, encounterId?: string) => Observation;
}

const SAMPLE_DURATION_MS = 15_000;
const TARGET_FPS = 30;
const FRAME_INTERVAL_MS = 1000 / TARGET_FPS;
// Pulse band: 40–200 bpm → 0.67–3.33 Hz at 30 fps
const MIN_FREQ_HZ = 0.67;
const MAX_FREQ_HZ = 3.33;

function meanRedChannel(imageData: ImageData): number {
  let sum = 0;
  const { data } = imageData;
  const pixels = data.length / 4;
  for (let i = 0; i < data.length; i += 4) sum += data[i]; // R channel
  return sum / pixels;
}

// Simple DFT on signal buffer — returns dominant frequency in Hz
function dominantFrequency(samples: number[], sampleRate: number): { freq: number; amplitude: number } {
  const n = samples.length;
  const mean = samples.reduce((a, b) => a + b, 0) / n;
  const centered = samples.map(v => v - mean);

  let bestFreq = 0;
  let bestAmp = 0;
  const freqStep = sampleRate / n;

  for (let k = 1; k < n / 2; k++) {
    const freq = k * freqStep;
    if (freq < MIN_FREQ_HZ || freq > MAX_FREQ_HZ) continue;
    let re = 0, im = 0;
    for (let i = 0; i < n; i++) {
      const angle = (2 * Math.PI * k * i) / n;
      re += centered[i] * Math.cos(angle);
      im -= centered[i] * Math.sin(angle);
    }
    const amp = Math.sqrt(re * re + im * im);
    if (amp > bestAmp) {
      bestAmp = amp;
      bestFreq = freq;
    }
  }
  return { freq: bestFreq, amplitude: bestAmp };
}

export function useHeartRate(): UseHeartRateReturn {
  const [status, setStatus] = useState<HeartRateStatus>("idle");
  const [error, setError] = useState<string | null>(null);
  const [bpm, setBpm] = useState<number | null>(null);
  const [confidence, setConfidence] = useState<number | null>(null);
  const [progress, setProgress] = useState(0);

  const streamRef = useRef<MediaStream | null>(null);
  const rafRef = useRef<number | null>(null);
  const startTimeRef = useRef<number>(0);
  const samplesRef = useRef<number[]>([]);
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
    setBpm(null);
    setConfidence(null);
    setProgress(0);
    setStatus("requesting");
    samplesRef.current = [];

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode: "environment",
          width: { ideal: 320 },
          height: { ideal: 240 },
          // torch/fill-light requested via applyConstraints after track is live
        },
        audio: false,
      });
      streamRef.current = stream;

      // Enable torch if available
      const track = stream.getVideoTracks()[0];
      if (track.getCapabilities && (track.getCapabilities() as MediaTrackCapabilities & { torch?: boolean }).torch) {
        await track.applyConstraints({ advanced: [{ torch: true } as MediaTrackConstraintSet] });
      }

      const video = document.createElement("video");
      video.srcObject = stream;
      video.setAttribute("playsinline", "true");
      await video.play();

      const canvas = document.createElement("canvas");
      canvas.width = video.videoWidth || 320;
      canvas.height = video.videoHeight || 240;
      const ctx = canvas.getContext("2d", { willReadFrequently: true })!;

      setStatus("acquiring");
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
          samplesRef.current.push(meanRedChannel(frame));
        }

        // Switch to "measuring" label once we have enough samples for a first estimate
        if (samplesRef.current.length === Math.floor(TARGET_FPS * 3) && status !== "measuring") {
          setStatus("measuring");
        }

        if (elapsed < SAMPLE_DURATION_MS) {
          rafRef.current = requestAnimationFrame(tick);
        } else {
          // Analysis
          const { freq, amplitude } = dominantFrequency(samplesRef.current, TARGET_FPS);
          const measuredBpm = Math.round(freq * 60);
          // Confidence: normalised peak amplitude relative to mean signal
          const mean = samplesRef.current.reduce((a, b) => a + b, 0) / samplesRef.current.length;
          const conf = Math.min(1, amplitude / (mean * samplesRef.current.length * 0.05));

          track.applyConstraints({ advanced: [{ torch: false } as MediaTrackConstraintSet] }).catch(() => {});
          streamRef.current?.getTracks().forEach(t => t.stop());
          streamRef.current = null;

          setBpm(measuredBpm);
          setConfidence(conf);
          setProgress(100);
          setStatus("done");
        }
      };
      rafRef.current = requestAnimationFrame(tick);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Camera unavailable");
      setStatus("error");
    }
  }, [status]);

  const toObservation = useCallback(
    (sample: HeartRateSample, patientId?: string, encounterId?: string): Observation => ({
      resourceType: "Observation",
      status: "final",
      category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "vital-signs" }] }],
      code: { coding: [{ system: "http://loinc.org", code: "8867-4", display: "Heart rate" }], text: "Heart rate" },
      effectiveDateTime: sample.measuredAt,
      valueQuantity: { value: sample.bpm, unit: "/min", system: "http://unitsofmeasure.org", code: "/min" },
      method: { coding: [{ system: "http://snomed.info/sct", code: "767002", display: "PPG via smartphone camera" }] },
      note: [{ text: `Signal confidence: ${Math.round(sample.confidence * 100)}%` }],
      ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
      ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
    }),
    []
  );

  return { status, error, bpm, confidence, progress, start, stop, toObservation };
}
