/**
 * useSpeechToText — Voice dictation hook using the browser Web Speech API as
 * the primary path (offline, no PHI leaves the device) with an optional
 * cloud STT fallback via the tpt-interop /api/v1/ai/transcribe endpoint.
 *
 * Usage:
 *   const { transcript, isListening, start, stop, clear, supported } = useSpeechToText({
 *     language: 'en-NZ',
 *     onTranscript: (text) => setNoteBody(prev => prev + text),
 *   });
 *
 * The Web Speech API is supported natively in Chrome, Edge, and Safari.
 * On browsers that do not support it (Firefox, older WebViews), the hook
 * falls back to cloud transcription if cloudFallbackUrl is provided, otherwise
 * it returns supported=false and the UI should hide the microphone button.
 */

import { useCallback, useEffect, useRef, useState } from 'react'

interface SpeechToTextOptions {
  /** BCP 47 language tag. Defaults to 'en-NZ'. */
  language?: string
  /** Called with each recognised interim and final transcript segment. */
  onTranscript?: (text: string, isFinal: boolean) => void
  /** If set, audio is sent here when the Web Speech API is unavailable.
   *  Expects POST multipart/form-data with field "audio" (WebM/Opus blob). */
  cloudFallbackUrl?: string
  /** Silence timeout in milliseconds before auto-stopping. Default 5000ms. */
  silenceTimeoutMs?: number
}

interface SpeechToTextState {
  /** Accumulated transcript text for the current dictation session. */
  transcript: string
  /** Whether the microphone is currently active. */
  isListening: boolean
  /** True when the Web Speech API or cloud fallback is available. */
  supported: boolean
  /** Last error message, if any. */
  error: string | null
}

interface SpeechToTextControls {
  /** Start listening. Clears the previous transcript. */
  start: () => void
  /** Stop listening and finalise the transcript. */
  stop: () => void
  /** Clear the accumulated transcript. */
  clear: () => void
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type SpeechRecognitionCtor = new () => any

function getSpeechRecognition(): SpeechRecognitionCtor | null {
  if (typeof window === 'undefined') return null
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const w = window as any
  return w.SpeechRecognition ?? w.webkitSpeechRecognition ?? null
}

export function useSpeechToText(
  options: SpeechToTextOptions = {}
): SpeechToTextState & SpeechToTextControls {
  const {
    language = 'en-NZ',
    onTranscript,
    cloudFallbackUrl,
    silenceTimeoutMs = 5000,
  } = options

  const SpeechRecognition = getSpeechRecognition()
  const hasNativeSTT = SpeechRecognition !== null
  const hasCloudFallback = Boolean(cloudFallbackUrl)
  const supported = hasNativeSTT || hasCloudFallback

  const [transcript, setTranscript] = useState('')
  const [isListening, setIsListening] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const recognitionRef = useRef<any>(null)
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const silenceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const clearSilenceTimer = useCallback(() => {
    if (silenceTimerRef.current) {
      clearTimeout(silenceTimerRef.current)
      silenceTimerRef.current = null
    }
  }, [])

  // --- Web Speech API path ---

  const startNative = useCallback(() => {
    if (!SpeechRecognition) return
    const recognition = new SpeechRecognition()
    recognitionRef.current = recognition
    recognition.lang = language
    recognition.continuous = true
    recognition.interimResults = true

    recognition.onresult = (event: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
      clearSilenceTimer()
      let interim = ''
      let final = ''
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const r = event.results[i]
        if (r.isFinal) {
          final += r[0].transcript
        } else {
          interim += r[0].transcript
        }
      }
      if (final) {
        setTranscript(prev => prev + final + ' ')
        onTranscript?.(final, true)
      } else if (interim) {
        onTranscript?.(interim, false)
      }
      // Auto-stop after silence
      silenceTimerRef.current = setTimeout(() => {
        recognition.stop()
      }, silenceTimeoutMs)
    }

    recognition.onerror = (event: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
      setError(`Speech recognition error: ${event.error}`)
      setIsListening(false)
      clearSilenceTimer()
    }

    recognition.onend = () => {
      setIsListening(false)
      clearSilenceTimer()
    }

    recognition.start()
    setIsListening(true)
    setError(null)
  }, [SpeechRecognition, language, onTranscript, silenceTimeoutMs, clearSilenceTimer])

  const stopNative = useCallback(() => {
    recognitionRef.current?.stop()
    clearSilenceTimer()
  }, [clearSilenceTimer])

  // --- Cloud STT fallback path (MediaRecorder → WebM/Opus → server) ---

  const startCloud = useCallback(async () => {
    if (!cloudFallbackUrl) return
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const recorder = new MediaRecorder(stream, { mimeType: 'audio/webm;codecs=opus' })
      mediaRecorderRef.current = recorder
      audioChunksRef.current = []

      recorder.ondataavailable = (e) => {
        if (e.data.size > 0) audioChunksRef.current.push(e.data)
      }

      recorder.onstop = async () => {
        stream.getTracks().forEach(t => t.stop())
        const blob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        const form = new FormData()
        form.append('audio', blob, 'dictation.webm')
        form.append('language', language)
        try {
          const res = await fetch(cloudFallbackUrl, { method: 'POST', body: form })
          if (!res.ok) throw new Error(`Server returned ${res.status}`)
          const { text } = await res.json() as { text: string }
          setTranscript(prev => prev + text + ' ')
          onTranscript?.(text, true)
        } catch (err) {
          setError(`Cloud STT failed: ${err instanceof Error ? err.message : String(err)}`)
        }
        setIsListening(false)
      }

      recorder.start()
      setIsListening(true)
      setError(null)

      // Auto-stop after silenceTimeoutMs of total recording
      silenceTimerRef.current = setTimeout(() => {
        recorder.stop()
        clearSilenceTimer()
      }, silenceTimeoutMs * 6) // give more time for cloud since it's batch, not streaming
    } catch (err) {
      setError(`Microphone access denied: ${err instanceof Error ? err.message : String(err)}`)
    }
  }, [cloudFallbackUrl, language, onTranscript, silenceTimeoutMs, clearSilenceTimer])

  const stopCloud = useCallback(() => {
    mediaRecorderRef.current?.stop()
    clearSilenceTimer()
  }, [clearSilenceTimer])

  // --- Public controls ---

  const start = useCallback(() => {
    setTranscript('')
    setError(null)
    if (hasNativeSTT) {
      startNative()
    } else if (hasCloudFallback) {
      startCloud()
    }
  }, [hasNativeSTT, hasCloudFallback, startNative, startCloud])

  const stop = useCallback(() => {
    if (hasNativeSTT) {
      stopNative()
    } else {
      stopCloud()
    }
  }, [hasNativeSTT, stopNative, stopCloud])

  const clear = useCallback(() => {
    setTranscript('')
    setError(null)
  }, [])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      recognitionRef.current?.stop()
      mediaRecorderRef.current?.stop()
      clearSilenceTimer()
    }
  }, [clearSilenceTimer])

  return { transcript, isListening, supported, error, start, stop, clear }
}
