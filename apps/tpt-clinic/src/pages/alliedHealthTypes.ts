export type Tab = 'plans' | 'claims' | 'sessions' | 'dashboard';
export type ToastSeverity = 'success' | 'error' | 'info' | 'warning';

export interface TreatmentPlan {
  id: string;
  patientNHI: string;
  patientName: string;
  clinician: string;
  profession: 'physiotherapy' | 'occupational_therapy' | 'speech_language_therapy' | 'podiatry';
  diagnosis: string;
  status: 'draft' | 'active' | 'under_review' | 'completed' | 'discontinued' | 'on_hold';
  startDate: string;
  reviewDate: string;
  accNumber?: string;
  sessionsUsed: number;
  sessionsApproved: number;
}

export interface ACCClaim {
  id: string;
  patientNHI: string;
  patientName: string;
  claimType: string;
  accNumber: string;
  status: 'draft' | 'submitted' | 'accepted' | 'declined' | 'under_review' | 'closed' | 'expired';
  diagnosis: string;
  bodyRegion: string;
  approvedSessions: number;
  usedSessions: number;
  startDate: string;
  expiryDate: string;
}

export interface SessionNote {
  id: string;
  patientNHI: string;
  patientName: string;
  clinician: string;
  profession: string;
  sessionDate: string;
  sessionNumber: number;
  durationMinutes: number;
  chargeCode: string;
  status: 'planned' | 'active' | 'completed' | 'cancelled';
}

export const professionClasses: Record<string, string> = {
  physiotherapy: 'bg-blue-50 text-blue-700 border border-blue-200',
  occupational_therapy: 'bg-green-50 text-green-700 border border-green-200',
  speech_language_therapy: 'bg-orange-50 text-orange-700 border border-orange-200',
  podiatry: 'bg-purple-50 text-purple-700 border border-purple-200',
};

export const professionProgressColor: Record<string, string> = {
  physiotherapy: 'bg-blue-500',
  occupational_therapy: 'bg-green-500',
  speech_language_therapy: 'bg-orange-500',
  podiatry: 'bg-purple-500',
};

export const professionLabels: Record<string, string> = {
  physiotherapy: 'Physiotherapy',
  occupational_therapy: 'Occupational Therapy',
  speech_language_therapy: 'Speech-Language Therapy',
  podiatry: 'Podiatry',
};

export const statusClasses: Record<string, string> = {
  draft: 'bg-secondary-100 text-secondary-600',
  active: 'bg-blue-100 text-blue-800',
  under_review: 'bg-amber-100 text-amber-800',
  completed: 'bg-green-100 text-green-800',
  discontinued: 'bg-red-100 text-red-800',
  on_hold: 'bg-sky-100 text-sky-800',
  submitted: 'bg-blue-100 text-blue-800',
  accepted: 'bg-green-100 text-green-800',
  declined: 'bg-red-100 text-red-800',
  closed: 'bg-secondary-100 text-secondary-600',
  expired: 'bg-red-100 text-red-800',
  planned: 'bg-secondary-100 text-secondary-600',
  cancelled: 'bg-red-100 text-red-800',
};

export const mockTreatmentPlans: TreatmentPlan[] = [
  {
    id: 'tp-001',
    patientNHI: 'ABC1234',
    patientName: 'John Smith',
    clinician: 'Dr. Sarah Wilson',
    profession: 'physiotherapy',
    diagnosis: 'Lumbar disc herniation L4/L5',
    status: 'active',
    startDate: '2024-01-15',
    reviewDate: '2024-03-15',
    accNumber: 'ACC123456',
    sessionsUsed: 6,
    sessionsApproved: 12,
  },
  {
    id: 'tp-002',
    patientNHI: 'DEF5678',
    patientName: 'Mary Johnson',
    clinician: 'Emma Thompson',
    profession: 'occupational_therapy',
    diagnosis: 'Stroke rehabilitation - right hemiplegia',
    status: 'active',
    startDate: '2024-02-01',
    reviewDate: '2024-04-01',
    accNumber: 'ACC789012',
    sessionsUsed: 8,
    sessionsApproved: 20,
  },
  {
    id: 'tp-003',
    patientNHI: 'GHI9012',
    patientName: 'Robert Brown',
    clinician: 'Dr. James Chen',
    profession: 'speech_language_therapy',
    diagnosis: 'Aphasia post-stroke',
    status: 'under_review',
    startDate: '2024-01-20',
    reviewDate: '2024-03-20',
    accNumber: 'ACC345678',
    sessionsUsed: 10,
    sessionsApproved: 15,
  },
  {
    id: 'tp-004',
    patientNHI: 'JKL3456',
    patientName: 'Susan Davis',
    clinician: 'Lisa Anderson',
    profession: 'podiatry',
    diagnosis: 'Diabetic foot ulcer - plantar forefoot',
    status: 'active',
    startDate: '2024-02-10',
    reviewDate: '2024-03-10',
    accNumber: 'ACC901234',
    sessionsUsed: 4,
    sessionsApproved: 10,
  },
];

export const mockACCClaims: ACCClaim[] = [
  {
    id: 'acc-001',
    patientNHI: 'ABC1234',
    patientName: 'John Smith',
    claimType: 'physiotherapy',
    accNumber: 'ACC123456',
    status: 'accepted',
    diagnosis: 'Lumbar disc herniation L4/L5',
    bodyRegion: 'lumbar_spine',
    approvedSessions: 12,
    usedSessions: 6,
    startDate: '2024-01-15',
    expiryDate: '2024-07-15',
  },
  {
    id: 'acc-002',
    patientNHI: 'DEF5678',
    patientName: 'Mary Johnson',
    claimType: 'occupational_therapy',
    accNumber: 'ACC789012',
    status: 'accepted',
    diagnosis: 'Stroke rehabilitation - right hemiplegia',
    bodyRegion: 'upper_limb',
    approvedSessions: 20,
    usedSessions: 8,
    startDate: '2024-02-01',
    expiryDate: '2024-08-01',
  },
  {
    id: 'acc-003',
    patientNHI: 'GHI9012',
    patientName: 'Robert Brown',
    claimType: 'speech_language_therapy',
    accNumber: 'ACC345678',
    status: 'under_review',
    diagnosis: 'Aphasia post-stroke',
    bodyRegion: 'cognitive_communication',
    approvedSessions: 15,
    usedSessions: 10,
    startDate: '2024-01-20',
    expiryDate: '2024-07-20',
  },
  {
    id: 'acc-004',
    patientNHI: 'JKL3456',
    patientName: 'Susan Davis',
    claimType: 'podiatry',
    accNumber: 'ACC901234',
    status: 'accepted',
    diagnosis: 'Diabetic foot ulcer - plantar forefoot',
    bodyRegion: 'foot',
    approvedSessions: 10,
    usedSessions: 4,
    startDate: '2024-02-10',
    expiryDate: '2024-08-10',
  },
];

export const mockSessionNotes: SessionNote[] = [
  {
    id: 'sn-001',
    patientNHI: 'ABC1234',
    patientName: 'John Smith',
    clinician: 'Dr. Sarah Wilson',
    profession: 'physiotherapy',
    sessionDate: '2024-02-20',
    sessionNumber: 6,
    durationMinutes: 30,
    chargeCode: 'PHY002',
    status: 'completed',
  },
  {
    id: 'sn-002',
    patientNHI: 'DEF5678',
    patientName: 'Mary Johnson',
    clinician: 'Emma Thompson',
    profession: 'occupational_therapy',
    sessionDate: '2024-02-21',
    sessionNumber: 8,
    durationMinutes: 45,
    chargeCode: 'OT002',
    status: 'completed',
  },
  {
    id: 'sn-003',
    patientNHI: 'GHI9012',
    patientName: 'Robert Brown',
    clinician: 'Dr. James Chen',
    profession: 'speech_language_therapy',
    sessionDate: '2024-02-22',
    sessionNumber: 10,
    durationMinutes: 45,
    chargeCode: 'SLT002',
    status: 'completed',
  },
  {
    id: 'sn-004',
    patientNHI: 'JKL3456',
    patientName: 'Susan Davis',
    clinician: 'Lisa Anderson',
    profession: 'podiatry',
    sessionDate: '2024-02-23',
    sessionNumber: 4,
    durationMinutes: 30,
    chargeCode: 'POD003',
    status: 'completed',
  },
];
