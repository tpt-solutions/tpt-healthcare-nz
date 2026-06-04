import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

interface TimeSlot {
  time: string;
  available: boolean;
}

interface Appointment {
  id: string;
  practitionerName: string;
  startTime: string;
  endTime: string;
  reason: string;
}

const TIMES: TimeSlot[] = [
  { time: '08:00', available: true },
  { time: '08:30', available: false },
  { time: '09:00', available: true },
  { time: '09:30', available: true },
  { time: '10:00', available: false },
  { time: '10:30', available: true },
  { time: '11:00', available: true },
  { time: '11:30', available: false },
  { time: '14:00', available: true },
  { time: '14:30', available: true },
  { time: '15:00', available: false },
  { time: '15:30', available: true },
];

type Step = 'date' | 'time' | 'reason' | 'confirm' | 'success';

export function BookAppointmentPage() {
  const navigate = useNavigate();
  const [step, setStep] = useState<Step>('date');
  const [selectedDate, setSelectedDate] = useState('');
  const [selectedTime, setSelectedTime] = useState('');
  const [reason, setReason] = useState('');
  const [booking, setBooking] = useState(false);
  const [booked, setBooked] = useState<Appointment | null>(null);
  const [error, setError] = useState('');

  // Minimum selectable date is tomorrow
  const tomorrow = new Date();
  tomorrow.setDate(tomorrow.getDate() + 1);
  const minDate = tomorrow.toISOString().split('T')[0];

  const maxDate = new Date();
  maxDate.setDate(maxDate.getDate() + 60);
  const maxDateStr = maxDate.toISOString().split('T')[0];

  const handleBook = async () => {
    if (!selectedDate || !selectedTime) return;
    setBooking(true);
    setError('');
    try {
      const res = await fetch('/api/v1/appointments', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          startTime: `${selectedDate}T${selectedTime}:00`,
          reason,
        }),
      });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Booking failed');
      }
      const data: Appointment = await res.json();
      setBooked(data);
      setStep('success');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Booking failed. Please try again.');
    } finally {
      setBooking(false);
    }
  };

  if (step === 'success' && booked) {
    return (
      <div className="p-6 max-w-md mx-auto">
        <div className="bg-white rounded-2xl border border-gray-200 p-8 text-center">
          <div className="h-16 w-16 rounded-full bg-teal-100 flex items-center justify-center mx-auto mb-4">
            <svg className="h-8 w-8 text-teal-600" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
            </svg>
          </div>
          <h2 className="text-xl font-bold text-gray-900 mb-2">Appointment booked</h2>
          <p className="text-gray-500 text-sm mb-1">{booked.practitionerName}</p>
          <p className="text-gray-700 font-medium">
            {new Date(booked.startTime).toLocaleDateString('en-NZ', {
              weekday: 'long', day: 'numeric', month: 'long',
            })} at {new Date(booked.startTime).toLocaleTimeString('en-NZ', {
              hour: '2-digit', minute: '2-digit', hour12: true,
            })}
          </p>
          <p className="text-xs text-gray-400 mt-4">
            You'll receive a reminder notification 24 hours and 1 hour before your appointment.
          </p>
          <button
            onClick={() => navigate('/appointments')}
            className="mt-6 w-full py-3 bg-teal-600 text-white font-semibold rounded-xl hover:bg-teal-700 transition-colors"
          >
            View my appointments
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 max-w-md mx-auto">
      <div className="flex items-center gap-3 mb-6">
        <button onClick={() => (step === 'date' ? navigate('/appointments') : setStep('date'))}
          className="text-gray-400 hover:text-gray-600">
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" />
          </svg>
        </button>
        <h1 className="text-2xl font-bold text-gray-900">Book appointment</h1>
      </div>

      {/* Step: date */}
      {step === 'date' && (
        <div className="bg-white rounded-2xl border border-gray-200 p-6">
          <label className="block text-sm font-medium text-gray-700 mb-2">Select a date</label>
          <input
            type="date"
            min={minDate}
            max={maxDateStr}
            value={selectedDate}
            onChange={(e) => setSelectedDate(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-gray-900 focus:outline-none focus:ring-2 focus:ring-teal-500"
          />
          <button
            disabled={!selectedDate}
            onClick={() => setStep('time')}
            className="mt-4 w-full py-3 bg-teal-600 text-white font-semibold rounded-xl hover:bg-teal-700 disabled:opacity-40 transition-colors"
          >
            Next — choose time
          </button>
        </div>
      )}

      {/* Step: time */}
      {step === 'time' && (
        <div className="bg-white rounded-2xl border border-gray-200 p-6">
          <p className="text-sm text-gray-500 mb-4">
            {new Date(selectedDate + 'T12:00:00').toLocaleDateString('en-NZ', {
              weekday: 'long', day: 'numeric', month: 'long',
            })}
          </p>
          <div className="grid grid-cols-3 gap-2">
            {TIMES.map(({ time, available }) => (
              <button
                key={time}
                disabled={!available}
                onClick={() => setSelectedTime(time)}
                className={`py-2.5 rounded-lg text-sm font-medium transition-colors ${
                  selectedTime === time
                    ? 'bg-teal-600 text-white'
                    : available
                    ? 'bg-gray-50 text-gray-700 hover:bg-teal-50 hover:text-teal-700 border border-gray-200'
                    : 'bg-gray-50 text-gray-300 cursor-not-allowed border border-gray-100'
                }`}
              >
                {time}
              </button>
            ))}
          </div>
          <button
            disabled={!selectedTime}
            onClick={() => setStep('reason')}
            className="mt-4 w-full py-3 bg-teal-600 text-white font-semibold rounded-xl hover:bg-teal-700 disabled:opacity-40 transition-colors"
          >
            Next — reason for visit
          </button>
        </div>
      )}

      {/* Step: reason */}
      {step === 'reason' && (
        <div className="bg-white rounded-2xl border border-gray-200 p-6">
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Reason for visit <span className="text-gray-400 font-normal">(optional)</span>
          </label>
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={3}
            placeholder="e.g. follow-up, prescription renewal, feeling unwell…"
            className="w-full rounded-lg border border-gray-300 px-4 py-2.5 text-sm text-gray-900 focus:outline-none focus:ring-2 focus:ring-teal-500 resize-none"
          />
          <button
            onClick={() => setStep('confirm')}
            className="mt-4 w-full py-3 bg-teal-600 text-white font-semibold rounded-xl hover:bg-teal-700 transition-colors"
          >
            Review booking
          </button>
        </div>
      )}

      {/* Step: confirm */}
      {step === 'confirm' && (
        <div className="bg-white rounded-2xl border border-gray-200 p-6">
          <h2 className="font-semibold text-gray-900 mb-4">Confirm your appointment</h2>
          <dl className="space-y-3 text-sm">
            <div className="flex justify-between">
              <dt className="text-gray-500">Date</dt>
              <dd className="text-gray-900 font-medium">
                {new Date(selectedDate + 'T12:00:00').toLocaleDateString('en-NZ', {
                  weekday: 'long', day: 'numeric', month: 'long',
                })}
              </dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Time</dt>
              <dd className="text-gray-900 font-medium">{selectedTime}</dd>
            </div>
            {reason && (
              <div className="flex justify-between">
                <dt className="text-gray-500">Reason</dt>
                <dd className="text-gray-900 max-w-[60%] text-right">{reason}</dd>
              </div>
            )}
          </dl>
          {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
          <button
            onClick={handleBook}
            disabled={booking}
            className="mt-6 w-full py-3 bg-teal-600 text-white font-semibold rounded-xl hover:bg-teal-700 disabled:opacity-60 transition-colors"
          >
            {booking ? 'Booking…' : 'Confirm booking'}
          </button>
        </div>
      )}
    </div>
  );
}
