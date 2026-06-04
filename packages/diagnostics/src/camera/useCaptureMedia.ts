import { useCallback, useRef, useState } from "react";
import type { Observation } from "@tpt/fhir-types";

export type CaptureKind = "wound" | "iris" | "eye" | "skin" | "generic";

export interface CaptureResult {
  dataUrl: string;
  mimeType: string;
  width: number;
  height: number;
  capturedAt: string;
  kind: CaptureKind;
}

export type CaptureStatus = "idle" | "requesting" | "previewing" | "captured" | "error";

export interface UseCaptureMediaReturn {
  status: CaptureStatus;
  error: string | null;
  preview: string | null;
  videoRef: React.RefObject<HTMLVideoElement>;
  start: (kind?: CaptureKind) => Promise<void>;
  capture: () => CaptureResult | null;
  stop: () => void;
  toObservation: (result: CaptureResult, patientId?: string, encounterId?: string) => Observation;
}

// LOINC codes for body-site imaging
const LOINC_BY_KIND: Record<CaptureKind, { code: string; display: string }> = {
  wound:   { code: "72170-4", display: "Photographic image [Attachment]" },
  iris:    { code: "29308-4", display: "Diagnosis" },
  eye:     { code: "29308-4", display: "Eye photo" },
  skin:    { code: "72170-4", display: "Skin lesion photo" },
  generic: { code: "72170-4", display: "Photographic image" },
};

export function useCaptureMedia(): UseCaptureMediaReturn {
  const [status, setStatus] = useState<CaptureStatus>("idle");
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const kindRef = useRef<CaptureKind>("generic");

  const stop = useCallback(() => {
    streamRef.current?.getTracks().forEach(t => t.stop());
    streamRef.current = null;
    setStatus("idle");
    setPreview(null);
  }, []);

  const start = useCallback(async (kind: CaptureKind = "generic") => {
    kindRef.current = kind;
    setError(null);
    setStatus("requesting");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "environment", width: { ideal: 1920 }, height: { ideal: 1080 } },
        audio: false,
      });
      streamRef.current = stream;
      if (videoRef.current) {
        videoRef.current.srcObject = stream;
        await videoRef.current.play();
      }
      setStatus("previewing");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Camera unavailable");
      setStatus("error");
    }
  }, []);

  const capture = useCallback((): CaptureResult | null => {
    const video = videoRef.current;
    if (!video || !streamRef.current) return null;

    const canvas = document.createElement("canvas");
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const ctx = canvas.getContext("2d");
    if (!ctx) return null;
    ctx.drawImage(video, 0, 0);

    const mimeType = "image/jpeg";
    const dataUrl = canvas.toDataURL(mimeType, 0.92);
    const result: CaptureResult = {
      dataUrl,
      mimeType,
      width: canvas.width,
      height: canvas.height,
      capturedAt: new Date().toISOString(),
      kind: kindRef.current,
    };
    setPreview(dataUrl);
    setStatus("captured");
    return result;
  }, []);

  const toObservation = useCallback(
    (result: CaptureResult, patientId?: string, encounterId?: string): Observation => {
      const loinc = LOINC_BY_KIND[result.kind];
      const obs: Observation = {
        resourceType: "Observation",
        status: "final",
        category: [{ coding: [{ system: "http://terminology.hl7.org/CodeSystem/observation-category", code: "imaging" }] }],
        code: { coding: [{ system: "http://loinc.org", code: loinc.code, display: loinc.display }], text: loinc.display },
        effectiveDateTime: result.capturedAt,
        valueString: result.dataUrl,
        note: [{ text: `${result.kind} photo — ${result.width}×${result.height}px` }],
        ...(patientId ? { subject: { reference: `Patient/${patientId}` } } : {}),
        ...(encounterId ? { encounter: { reference: `Encounter/${encounterId}` } } : {}),
      };
      return obs;
    },
    []
  );

  return { status, error, preview, videoRef, start, capture, stop, toObservation };
}
