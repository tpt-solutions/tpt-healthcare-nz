import { useState } from 'react';

interface Message {
  id: string;
  subject: string;
  from: string;
  fromRole: string;
  date: string;
  preview: string;
  body: string;
  read: boolean;
  direction: 'inbound' | 'outbound';
}

const messages: Message[] = [
  {
    id: 'msg-1',
    subject: 'Your HbA1c results',
    from: 'Dr. Hemi Walker',
    fromRole: 'General Practitioner',
    date: '2026-05-22T10:15:00',
    preview: 'Your HbA1c result from your recent blood test has come through...',
    body: `Your HbA1c result from your recent blood test has come through at 52 mmol/mol. This is slightly above the target range of 48 mmol/mol.\n\nThis is not an urgent concern, but I would like to review your diet and medication at your next appointment on 10 June.\n\nIn the meantime, please continue your current Metformin dose and try to reduce your intake of refined carbohydrates.\n\nKind regards,\nDr. Hemi Walker`,
    read: false,
    direction: 'inbound',
  },
  {
    id: 'msg-2',
    subject: 'Appointment reminder — 10 June 2026',
    from: 'Auckland City Medical Centre',
    fromRole: 'Reception',
    date: '2026-06-01T08:00:00',
    preview: 'This is a reminder for your appointment with Dr. Walker on 10 June 2026 at 09:30.',
    body: `This is a reminder for your appointment with Dr. Walker on 10 June 2026 at 09:30 am.\n\nLocation: Auckland City Medical Centre, Level 2, 123 Queen Street, Auckland CBD.\n\nPlease arrive 5 minutes early to allow time for check-in. If you need to cancel or reschedule, please do so at least 24 hours before your appointment.\n\nThank you.`,
    read: true,
    direction: 'inbound',
  },
  {
    id: 'msg-3',
    subject: 'Question about Lisinopril',
    from: 'Aroha Ngata',
    fromRole: 'Patient',
    date: '2026-05-15T14:30:00',
    preview: 'Hi Dr. Walker, I wanted to ask about a side effect I have been experiencing...',
    body: `Hi Dr. Walker,\n\nI wanted to ask about a side effect I have been experiencing since starting Lisinopril. I have a dry cough that does not seem to be going away.\n\nIs this normal? Should I be concerned?\n\nThanks,\nAroha`,
    read: true,
    direction: 'outbound',
  },
];

function formatDateTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString('en-NZ', { day: 'numeric', month: 'short', year: 'numeric' }) +
    ' at ' + d.toLocaleTimeString('en-NZ', { hour: '2-digit', minute: '2-digit' });
}

export function MessagesPage() {
  const [selected, setSelected] = useState<Message | null>(null);
  const [showCompose, setShowCompose] = useState(false);
  const [composeSubject, setComposeSubject] = useState('');
  const [composeBody, setComposeBody] = useState('');

  const unreadCount = messages.filter(m => !m.read).length;

  return (
    <div className="p-6 max-w-5xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">
            Secure Messages
            {unreadCount > 0 && (
              <span className="ml-2 inline-flex items-center rounded-full bg-brand-600 px-2 py-0.5 text-xs font-medium text-white">
                {unreadCount} new
              </span>
            )}
          </h1>
          <p className="mt-1 text-sm text-gray-500">
            Encrypted messages between you and your care team.
          </p>
        </div>
        <button
          onClick={() => setShowCompose(true)}
          className="inline-flex items-center gap-2 rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-700 transition-colors"
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          New Message
        </button>
      </div>

      {/* Encryption notice */}
      <div className="bg-green-50 border border-green-200 rounded-xl px-4 py-3 mb-5 flex items-center gap-3">
        <svg className="h-4 w-4 text-green-600 flex-shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
        </svg>
        <p className="text-xs text-green-700">
          All messages are end-to-end encrypted and stored in compliance with HIPC Rule 5. Only you and your care team can read them.
        </p>
      </div>

      <div className="flex gap-4 h-[calc(100vh-300px)] min-h-96">
        {/* Message list */}
        <div className="w-80 flex-shrink-0 bg-white rounded-xl border border-gray-200 shadow-sm overflow-auto">
          {messages.map(msg => (
            <button
              key={msg.id}
              onClick={() => setSelected(msg)}
              className={`w-full text-left px-4 py-4 border-b border-gray-100 hover:bg-gray-50 transition-colors ${
                selected?.id === msg.id ? 'bg-brand-50 border-l-2 border-l-brand-500' : ''
              }`}
            >
              <div className="flex items-start justify-between gap-2">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    {!msg.read && (
                      <span className="h-2 w-2 rounded-full bg-brand-500 flex-shrink-0" />
                    )}
                    <p className={`text-sm truncate ${!msg.read ? 'font-semibold text-gray-900' : 'font-medium text-gray-700'}`}>
                      {msg.subject}
                    </p>
                  </div>
                  <p className="text-xs text-gray-500 mt-0.5">
                    {msg.direction === 'outbound' ? `To: ${msg.from}` : msg.from}
                  </p>
                  <p className="text-xs text-gray-400 mt-0.5 line-clamp-1">{msg.preview}</p>
                </div>
              </div>
              <p className="text-xs text-gray-400 mt-1 text-right">
                {new Date(msg.date).toLocaleDateString('en-NZ', { day: 'numeric', month: 'short' })}
              </p>
            </button>
          ))}
        </div>

        {/* Message body */}
        <div className="flex-1 bg-white rounded-xl border border-gray-200 shadow-sm overflow-auto">
          {selected ? (
            <div className="p-6">
              <div className="border-b border-gray-100 pb-4 mb-5">
                <h2 className="text-base font-semibold text-gray-900">{selected.subject}</h2>
                <p className="text-sm text-gray-600 mt-1">
                  {selected.direction === 'outbound' ? (
                    <>To: <span className="font-medium">{selected.from}</span> ({selected.fromRole})</>
                  ) : (
                    <>From: <span className="font-medium">{selected.from}</span> ({selected.fromRole})</>
                  )}
                </p>
                <p className="text-xs text-gray-400 mt-1">{formatDateTime(selected.date)}</p>
              </div>
              <div className="prose prose-sm max-w-none">
                {selected.body.split('\n').map((line, i) => (
                  <p key={i} className="text-sm text-gray-700 mb-2 empty:mb-0">{line}</p>
                ))}
              </div>
            </div>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-400">
              <p className="text-sm">Select a message to read it</p>
            </div>
          )}
        </div>
      </div>

      {/* Compose modal */}
      {showCompose && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-2xl shadow-xl p-6 w-full max-w-lg mx-4">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-base font-semibold text-gray-900">New Message</h2>
              <button
                onClick={() => setShowCompose(false)}
                className="text-gray-400 hover:text-gray-600"
              >
                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">To</label>
                <select className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none">
                  <option>Dr. Hemi Walker</option>
                  <option>Auckland City Medical Centre Reception</option>
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Subject</label>
                <input
                  type="text"
                  value={composeSubject}
                  onChange={e => setComposeSubject(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none"
                  placeholder="Message subject..."
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Message</label>
                <textarea
                  rows={6}
                  value={composeBody}
                  onChange={e => setComposeBody(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none resize-none"
                  placeholder="Write your message here..."
                />
              </div>
            </div>
            <p className="mt-3 text-xs text-gray-400">
              This message will be encrypted and sent securely to your care team.
            </p>
            <div className="flex gap-3 mt-4">
              <button
                onClick={() => setShowCompose(false)}
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => setShowCompose(false)}
                className="flex-1 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 transition-colors"
              >
                Send
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
