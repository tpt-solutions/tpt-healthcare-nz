import React, { useRef, useState } from 'react';
import AppShell from '@/components/AppShell';
import {
  useHeartRate,
  useCaptureMedia,
  useTremor,
  useGait,
  useBalance,
  useVisualAcuity,
  useColourVision,
  useReactionTime,
  useHearingScreen,
  useOximeter,
  useBloodPressure,
} from '@tpt/diagnostics';

// ---------------------------------------------------------------------------
// Shared UI primitives
// ---------------------------------------------------------------------------

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-xl border border-secondary-200 bg-white p-4 shadow-sm">
      <h2 className="mb-3 text-base font-semibold text-secondary-900">{title}</h2>
      {children}
    </section>
  );
}

function StatusBadge({ label, color = 'gray' }: { label: string; color?: 'gray' | 'green' | 'amber' | 'red' | 'blue' }) {
  const colors = {
    gray:  'bg-secondary-100 text-secondary-700',
    green: 'bg-green-100 text-green-800',
    amber: 'bg-amber-100 text-amber-800',
    red:   'bg-red-100 text-red-800',
    blue:  'bg-blue-100 text-blue-800',
  };
  return <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${colors[color]}`}>{label}</span>;
}

function Btn({
  onClick, disabled, children, variant = 'primary',
}: {
  onClick: () => void; disabled?: boolean; children: React.ReactNode; variant?: 'primary' | 'secondary' | 'danger';
}) {
  const base = 'rounded-md px-3 py-1.5 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-50';
  const variants = {
    primary: 'bg-primary-600 text-white hover:bg-primary-700',
    secondary: 'border border-secondary-300 bg-white text-secondary-700 hover:bg-secondary-50',
    danger: 'bg-red-600 text-white hover:bg-red-700',
  };
  return (
    <button className={`${base} ${variants[variant]}`} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  );
}

function ProgressBar({ value }: { value: number }) {
  return (
    <div className="h-2 w-full overflow-hidden rounded-full bg-secondary-100">
      <div className="h-full rounded-full bg-primary-500 transition-all" style={{ width: `${value}%` }} />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Heart Rate panel
// ---------------------------------------------------------------------------
function HeartRatePanel() {
  const hr = useHeartRate();
  return (
    <Section title="Heart Rate (PPG via camera)">
      <p className="mb-3 text-xs text-secondary-500">
        Cover the rear camera with your fingertip. Keep still for 15 seconds.
      </p>
      <div className="flex items-center gap-3 mb-3">
        <Btn onClick={hr.start} disabled={hr.status === 'acquiring' || hr.status === 'measuring'} variant="primary">
          {hr.status === 'idle' || hr.status === 'done' || hr.status === 'error' ? 'Start' : 'Measuring…'}
        </Btn>
        {(hr.status === 'acquiring' || hr.status === 'measuring') && (
          <Btn onClick={hr.stop} variant="danger">Cancel</Btn>
        )}
      </div>
      {(hr.status === 'acquiring' || hr.status === 'measuring') && <ProgressBar value={hr.progress} />}
      {hr.status === 'done' && hr.bpm !== null && (
        <div className="mt-2 flex items-center gap-3">
          <span className="text-3xl font-bold text-primary-600">{hr.bpm}</span>
          <span className="text-secondary-500">bpm</span>
          <StatusBadge
            label={`Confidence: ${Math.round((hr.confidence ?? 0) * 100)}%`}
            color={(hr.confidence ?? 0) > 0.6 ? 'green' : 'amber'}
          />
        </div>
      )}
      {hr.status === 'error' && <p className="mt-2 text-sm text-red-600">{hr.error}</p>}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Photo capture panel
// ---------------------------------------------------------------------------
function CaptureMediaPanel() {
  const cam = useCaptureMedia();
  const videoRef = cam.videoRef as React.RefObject<HTMLVideoElement>;
  return (
    <Section title="Photo Capture (wound, iris, skin)">
      <div className="flex flex-wrap gap-2 mb-3">
        {(['wound', 'iris', 'eye', 'skin'] as const).map(kind => (
          <Btn key={kind} onClick={() => cam.start(kind)} disabled={cam.status === 'previewing'} variant="secondary">
            {kind.charAt(0).toUpperCase() + kind.slice(1)}
          </Btn>
        ))}
        {cam.status === 'previewing' && (
          <>
            <Btn onClick={() => cam.capture()} variant="primary">Capture</Btn>
            <Btn onClick={cam.stop} variant="danger">Cancel</Btn>
          </>
        )}
      </div>
      {(cam.status === 'previewing') && (
        <video ref={videoRef} className="w-full max-w-sm rounded-lg border border-secondary-200" playsInline muted />
      )}
      {cam.status === 'captured' && cam.preview && (
        <img src={cam.preview} alt="Captured" className="mt-2 w-full max-w-sm rounded-lg border border-secondary-200" />
      )}
      {cam.status === 'error' && <p className="mt-2 text-sm text-red-600">{cam.error}</p>}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Tremor panel
// ---------------------------------------------------------------------------
function TremorPanel() {
  const tr = useTremor();
  const classColors: Record<string, 'green' | 'amber' | 'red' | 'gray'> = {
    normal: 'green', mild: 'amber', moderate: 'amber', severe: 'red',
  };
  return (
    <Section title="Tremor Assessment (accelerometer)">
      <p className="mb-3 text-xs text-secondary-500">
        Hold the phone in outstretched hand (action) or rest it on back of hand (resting).
      </p>
      <div className="flex gap-2 mb-3">
        <Btn onClick={() => tr.start('action')} disabled={tr.status === 'measuring'} variant="secondary">
          Action tremor (10 s)
        </Btn>
        <Btn onClick={() => tr.start('resting')} disabled={tr.status === 'measuring'} variant="secondary">
          Resting tremor (10 s)
        </Btn>
        {tr.status === 'measuring' && <Btn onClick={tr.stop} variant="danger">Cancel</Btn>}
      </div>
      {tr.status === 'measuring' && <ProgressBar value={tr.progress} />}
      {tr.status === 'done' && tr.result && (
        <div className="mt-2 space-y-1">
          <StatusBadge label={tr.result.classification} color={classColors[tr.result.classification] ?? 'gray'} />
          <p className="text-xs text-secondary-600">
            Dominant frequency: {tr.result.dominantFreqHz} Hz &nbsp;|&nbsp; PSD: {tr.result.powerSpectralDensity}
          </p>
        </div>
      )}
      {tr.status === 'error' && <p className="mt-2 text-sm text-red-600">{tr.error}</p>}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Gait / TUG panel
// ---------------------------------------------------------------------------
function GaitPanel() {
  const g = useGait();
  const fallColors: Record<string, 'green' | 'amber' | 'red'> = { low: 'green', moderate: 'amber', high: 'red' };
  return (
    <Section title="Gait / TUG Test (accelerometer)">
      <p className="mb-3 text-xs text-secondary-500">
        Place phone in pocket. Tap Start, stand up, walk 3 m, turn, return, and sit. Tap Stop.
      </p>
      <div className="flex gap-2 mb-3">
        <Btn onClick={g.startManual} disabled={g.status === 'measuring'} variant="primary">Start TUG</Btn>
        {g.status === 'measuring' && <Btn onClick={g.stopManual} variant="danger">Stop</Btn>}
      </div>
      {g.status === 'measuring' && (
        <StatusBadge label={`Phase: ${g.phase}`} color="blue" />
      )}
      {g.status === 'done' && g.result && (
        <div className="mt-2 space-y-1">
          <StatusBadge label={`Fall risk: ${g.result.fallRisk}`} color={fallColors[g.result.fallRisk]} />
          <p className="text-xs text-secondary-600">
            Duration: {(g.result.tugDurationMs / 1000).toFixed(1)} s &nbsp;|&nbsp;
            Steps: {g.result.stepCount} &nbsp;|&nbsp;
            Cadence: {g.result.cadenceStepsPerMin} /min &nbsp;|&nbsp;
            Symmetry: {g.result.symmetryIndex}
          </p>
        </div>
      )}
      {g.status === 'error' && <p className="mt-2 text-sm text-red-600">{g.error}</p>}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Balance / Romberg panel
// ---------------------------------------------------------------------------
function BalancePanel() {
  const b = useBalance();
  return (
    <Section title="Romberg Balance Test (gyroscope)">
      <p className="mb-3 text-xs text-secondary-500">
        Stand with feet together, phone in breast pocket. Two 30 s trials (eyes open then closed).
      </p>
      <div className="flex gap-2 mb-3">
        <Btn onClick={b.start} disabled={b.status !== 'idle' && b.status !== 'done'} variant="primary">
          {b.status === 'idle' || b.status === 'done' ? 'Start' : b.status.replace('-', ' ')}
        </Btn>
        {b.status !== 'idle' && b.status !== 'done' && (
          <Btn onClick={b.stop} variant="danger">Cancel</Btn>
        )}
      </div>
      {b.status !== 'idle' && b.status !== 'done' && <ProgressBar value={b.progress} />}
      {b.status === 'done' && b.result && (
        <div className="mt-2 space-y-1">
          <StatusBadge
            label={b.result.interpretation}
            color={b.result.interpretation === 'normal' ? 'green' : 'amber'}
          />
          <p className="text-xs text-secondary-600">
            Romberg ratio: {b.result.rombergRatio} &nbsp;|&nbsp;
            Sway open: {b.result.eyesOpen.swayDegrees}° &nbsp;|&nbsp;
            Sway closed: {b.result.eyesClosed.swayDegrees}°
          </p>
        </div>
      )}
      {b.status === 'error' && <p className="mt-2 text-sm text-red-600">{b.error}</p>}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Visual Acuity panel
// ---------------------------------------------------------------------------
function VisualAcuityPanel() {
  const va = useVisualAcuity();
  return (
    <Section title="Visual Acuity (Snellen)">
      <p className="mb-3 text-xs text-secondary-500">
        Hold phone at arm's length (~40 cm). Identify each letter.
      </p>
      {va.status === 'idle' && (
        <div className="flex gap-2">
          {(['left', 'right', 'both'] as const).map(eye => (
            <Btn key={eye} onClick={() => va.start(eye)} variant="secondary">
              {eye.charAt(0).toUpperCase() + eye.slice(1)} eye
            </Btn>
          ))}
        </div>
      )}
      {va.status === 'testing' && va.currentOptotype && (
        <div className="space-y-3">
          <div className="flex items-center justify-center rounded-lg bg-white py-6" style={{ fontSize: va.currentSizePx }}>
            <span className="font-mono font-bold text-secondary-900 select-none">{va.currentOptotype}</span>
          </div>
          <p className="text-center text-xs text-secondary-500">
            Line {va.lineIndex + 1} of {va.totalLines}
          </p>
          <div className="flex justify-center gap-3">
            <Btn onClick={() => va.recordResponse(true)} variant="primary">Correct</Btn>
            <Btn onClick={() => va.recordResponse(false)} variant="secondary">Incorrect</Btn>
          </div>
        </div>
      )}
      {va.status === 'done' && va.result && (
        <div className="mt-2 space-y-1">
          <StatusBadge label={va.result.snellen} color={va.result.logMAR <= 0 ? 'green' : va.result.logMAR <= 0.3 ? 'amber' : 'red'} />
          <p className="text-xs text-secondary-600">logMAR: {va.result.logMAR} &nbsp;|&nbsp; Eye: {va.result.eye}</p>
          <Btn onClick={() => va.start('both')} variant="secondary">Retest</Btn>
        </div>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Colour Vision panel
// ---------------------------------------------------------------------------
function ColourVisionPanel() {
  const cv = useColourVision();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  React.useEffect(() => {
    if (cv.currentPlateCanvas && canvasRef.current) {
      const ctx = canvasRef.current.getContext('2d');
      if (ctx) {
        canvasRef.current.width  = cv.currentPlateCanvas.width;
        canvasRef.current.height = cv.currentPlateCanvas.height;
        ctx.drawImage(cv.currentPlateCanvas, 0, 0);
      }
    }
  }, [cv.currentPlateCanvas]);

  return (
    <Section title="Colour Vision Screening (Ishihara-equivalent)">
      {cv.status === 'idle' && (
        <Btn onClick={cv.start} variant="primary">Start</Btn>
      )}
      {cv.status === 'testing' && (
        <div className="space-y-3">
          <canvas ref={canvasRef} className="w-full max-w-xs rounded-lg border border-secondary-200" />
          <p className="text-xs text-secondary-500">Plate {cv.plateIndex + 1} / {cv.totalPlates} — enter the number you see:</p>
          <div className="flex flex-wrap gap-2">
            {['0','1','2','3','4','5','6','7','8','9'].map(n => (
              <button key={n} onClick={() => cv.recordAnswer(n)} className="h-8 w-8 rounded-md border border-secondary-300 text-sm hover:bg-secondary-50">
                {n}
              </button>
            ))}
            <button onClick={() => cv.recordAnswer('')} className="rounded-md border border-secondary-300 px-2 text-xs text-secondary-500 hover:bg-secondary-50">
              None
            </button>
          </div>
        </div>
      )}
      {cv.status === 'done' && cv.result && (
        <div className="mt-2 space-y-1">
          <StatusBadge label={cv.result.classification} color={cv.result.classification === 'normal' ? 'green' : 'amber'} />
          <p className="text-xs text-secondary-600">Score: {cv.result.score}/{cv.totalPlates} &nbsp;|&nbsp; {cv.result.deficiencyType}</p>
          <Btn onClick={cv.start} variant="secondary">Retest</Btn>
        </div>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Reaction time panel
// ---------------------------------------------------------------------------
function ReactionTimePanel() {
  const rt = useReactionTime();
  const containerRef = useRef<HTMLDivElement>(null);

  function handleContainerClick(e: React.MouseEvent<HTMLDivElement>) {
    if (!containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();
    rt.recordTap(e.clientX - rect.left, e.clientY - rect.top, rect.width, rect.height);
  }

  return (
    <Section title="Reaction Time / Fine Motor">
      {(rt.status === 'idle' || rt.status === 'done') && (
        <div className="space-y-2">
          {rt.status === 'done' && rt.result && (
            <div className="mb-2 space-y-1">
              <StatusBadge label={rt.result.cognitiveScreen} color={rt.result.cognitiveScreen === 'normal' ? 'green' : rt.result.cognitiveScreen === 'borderline' ? 'amber' : 'red'} />
              <p className="text-xs text-secondary-600">
                Mean: {rt.result.meanReactionMs} ms &nbsp;|&nbsp;
                Median: {rt.result.medianReactionMs} ms &nbsp;|&nbsp;
                CV: {rt.result.cvPercent}% &nbsp;|&nbsp;
                Miss rate: {Math.round(rt.result.missRate * 100)}%
              </p>
            </div>
          )}
          <Btn onClick={() => rt.start(10)} variant="primary">Start (10 trials)</Btn>
        </div>
      )}
      {(rt.status === 'waiting' || rt.status === 'ready') && (
        <div
          ref={containerRef}
          className="relative w-full h-48 cursor-pointer rounded-lg bg-secondary-50 border border-secondary-200 select-none"
          onClick={handleContainerClick}
        >
          <p className="absolute top-2 left-3 text-xs text-secondary-400">Trial {rt.currentTrial + 1} / {rt.totalTrials}</p>
          {rt.status === 'waiting' && (
            <p className="flex h-full items-center justify-center text-sm text-secondary-400">Wait…</p>
          )}
          {rt.status === 'ready' && rt.targetX !== null && rt.targetY !== null && (
            <div
              className="absolute h-10 w-10 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-500"
              style={{ left: `${rt.targetX * 100}%`, top: `${rt.targetY * 100}%` }}
            />
          )}
        </div>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Hearing screen panel
// ---------------------------------------------------------------------------
function HearingScreenPanel() {
  const h = useHearingScreen();
  return (
    <Section title="Hearing Screening (pure-tone)">
      <p className="mb-3 text-xs text-secondary-500">
        Use headphones. Tap "Heard it" when you hear the tone, "Didn't hear" if not.
      </p>
      {h.status === 'idle' && <Btn onClick={h.start} variant="primary">Start</Btn>}
      {h.status === 'testing' && (
        <div className="space-y-3">
          <p className="text-sm text-secondary-700">
            {h.currentFreqHz} Hz &nbsp;|&nbsp; {h.currentEar} ear &nbsp;|&nbsp; ~{h.currentLevelDbHL} dBHL
          </p>
          <div className="flex gap-3">
            <Btn onClick={() => h.recordResponse(true)} variant="primary">Heard it</Btn>
            <Btn onClick={() => h.recordResponse(false)} variant="secondary">Didn't hear</Btn>
          </div>
        </div>
      )}
      {h.status === 'done' && h.result && (
        <div className="mt-2 space-y-2">
          <StatusBadge
            label={h.result.overallClassification}
            color={h.result.overallClassification === 'normal' ? 'green' : h.result.overallClassification === 'mild-loss' ? 'amber' : 'red'}
          />
          <div className="grid grid-cols-2 gap-2 text-xs text-secondary-600">
            <div>
              <p className="font-medium mb-1">Right ear</p>
              {h.result.rightEar.map(t => (
                <p key={t.freqHz}>{t.freqHz} Hz — {t.thresholdDbHL} dBHL ({t.category})</p>
              ))}
            </div>
            <div>
              <p className="font-medium mb-1">Left ear</p>
              {h.result.leftEar.map(t => (
                <p key={t.freqHz}>{t.freqHz} Hz — {t.thresholdDbHL} dBHL ({t.category})</p>
              ))}
            </div>
          </div>
          <Btn onClick={h.start} variant="secondary">Retest</Btn>
        </div>
      )}
      {h.status === 'error' && <p className="mt-2 text-sm text-red-600">{h.error}</p>}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// BLE Oximeter panel
// ---------------------------------------------------------------------------
function OximeterPanel() {
  const ox = useOximeter();
  const statusColors: Record<string, 'gray' | 'blue' | 'green' | 'red'> = {
    idle: 'gray', scanning: 'blue', connecting: 'blue', connected: 'green', disconnected: 'gray', error: 'red', unsupported: 'red',
  };
  return (
    <Section title="BLE Pulse Oximeter (Web Bluetooth)">
      <p className="mb-3 text-xs text-secondary-500">
        Requires a Bluetooth LE pulse oximeter (e.g. Wellue O2Ring, Nonin 3230).
      </p>
      <div className="flex items-center gap-3 mb-3">
        <StatusBadge label={ox.bleStatus} color={statusColors[ox.bleStatus] ?? 'gray'} />
        {ox.bleStatus === 'idle' || ox.bleStatus === 'disconnected' || ox.bleStatus === 'error'
          ? <Btn onClick={ox.connect} variant="primary">Pair device</Btn>
          : <Btn onClick={ox.disconnect} variant="danger">Disconnect</Btn>
        }
      </div>
      {ox.bleError && <p className="text-sm text-red-600 mb-2">{ox.bleError}</p>}
      {ox.reading && (
        <div className="flex items-center gap-6">
          <div>
            <p className="text-3xl font-bold text-primary-600">{ox.reading.spO2Percent}<span className="text-base font-normal text-secondary-500"> %</span></p>
            <p className="text-xs text-secondary-500">SpO₂</p>
          </div>
          <div>
            <p className="text-3xl font-bold text-secondary-700">{ox.reading.heartRateBpm}<span className="text-base font-normal text-secondary-500"> bpm</span></p>
            <p className="text-xs text-secondary-500">Heart rate</p>
          </div>
        </div>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// BLE Blood Pressure panel
// ---------------------------------------------------------------------------
function BloodPressurePanel() {
  const bp = useBloodPressure();
  const statusColors: Record<string, 'gray' | 'blue' | 'green' | 'red'> = {
    idle: 'gray', scanning: 'blue', connecting: 'blue', connected: 'green', disconnected: 'gray', error: 'red', unsupported: 'red',
  };
  const bpColors: Record<string, 'green' | 'amber' | 'red'> = {
    optimal: 'green', normal: 'green', 'high-normal': 'amber',
    'grade-1-hypertension': 'amber', 'grade-2-hypertension': 'red', 'grade-3-hypertension': 'red', 'isolated-systolic-hypertension': 'amber',
  };
  return (
    <Section title="BLE Blood Pressure (Web Bluetooth)">
      <p className="mb-3 text-xs text-secondary-500">
        Requires a Bluetooth LE BP cuff (e.g. OMRON BLE, iHealth Clear).
      </p>
      <div className="flex items-center gap-3 mb-3">
        <StatusBadge label={bp.bleStatus} color={statusColors[bp.bleStatus] ?? 'gray'} />
        {bp.bleStatus === 'idle' || bp.bleStatus === 'disconnected' || bp.bleStatus === 'error'
          ? <Btn onClick={bp.connect} variant="primary">Pair device</Btn>
          : <Btn onClick={bp.disconnect} variant="danger">Disconnect</Btn>
        }
      </div>
      {bp.bleError && <p className="text-sm text-red-600 mb-2">{bp.bleError}</p>}
      {bp.cuffPressure !== null && (
        <p className="mb-2 text-sm text-secondary-500 animate-pulse">Cuff inflating: {bp.cuffPressure} mmHg…</p>
      )}
      {bp.reading && (
        <div className="space-y-1">
          <div className="flex items-baseline gap-2">
            <span className="text-3xl font-bold text-primary-600">{bp.reading.systolicMmHg}</span>
            <span className="text-xl text-secondary-500">/</span>
            <span className="text-3xl font-bold text-secondary-700">{bp.reading.diastolicMmHg}</span>
            <span className="text-sm text-secondary-400">mmHg</span>
          </div>
          <StatusBadge label={bp.reading.classification} color={bpColors[bp.reading.classification] ?? 'gray'} />
          {bp.reading.pulseRateBpm !== null && (
            <p className="text-xs text-secondary-500">Pulse: {bp.reading.pulseRateBpm} bpm</p>
          )}
        </div>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export default function DiagnosticsPage() {
  const [activeTab, setActiveTab] = useState<'camera' | 'sensors' | 'screen' | 'audio' | 'bluetooth'>('camera');

  const tabs: Array<{ key: typeof activeTab; label: string }> = [
    { key: 'camera',    label: 'Camera' },
    { key: 'sensors',   label: 'Sensors' },
    { key: 'screen',    label: 'Screen' },
    { key: 'audio',     label: 'Audio' },
    { key: 'bluetooth', label: 'Bluetooth' },
  ];

  return (
    <AppShell title="Diagnostics">
      <div className="max-w-2xl space-y-4">
        <p className="text-sm text-secondary-500">
          Device-integrated diagnostic tools. All results emit FHIR R5 Observation resources
          ready to be posted to the encounter record.
        </p>

        {/* Tab bar */}
        <div className="flex gap-1 rounded-lg border border-secondary-200 bg-secondary-50 p-1">
          {tabs.map(t => (
            <button
              key={t.key}
              onClick={() => setActiveTab(t.key)}
              className={[
                'flex-1 rounded-md py-1.5 text-xs font-medium transition-colors',
                activeTab === t.key
                  ? 'bg-white text-secondary-900 shadow-sm'
                  : 'text-secondary-500 hover:text-secondary-700',
              ].join(' ')}
            >
              {t.label}
            </button>
          ))}
        </div>

        {activeTab === 'camera' && (
          <>
            <HeartRatePanel />
            <CaptureMediaPanel />
          </>
        )}
        {activeTab === 'sensors' && (
          <>
            <TremorPanel />
            <GaitPanel />
            <BalancePanel />
          </>
        )}
        {activeTab === 'screen' && (
          <>
            <VisualAcuityPanel />
            <ColourVisionPanel />
            <ReactionTimePanel />
          </>
        )}
        {activeTab === 'audio' && <HearingScreenPanel />}
        {activeTab === 'bluetooth' && (
          <>
            <OximeterPanel />
            <BloodPressurePanel />
          </>
        )}
      </div>
    </AppShell>
  );
}
