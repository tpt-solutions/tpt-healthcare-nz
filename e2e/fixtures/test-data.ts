export const TEST_PRACTITIONER = {
  email: 'e2e.practitioner@tpt.test',
  password: 'Test1234!',
  user: {
    id: 'practitioner-e2e-1',
    email: 'e2e.practitioner@tpt.test',
    name: 'Dr Jane Practitioner',
    hpiCpn: 'CPN12345',
    roles: ['gp'],
  },
  accessToken: 'e2e-fake-access-token',
} as const;

export const TEST_PATIENT = {
  id: 'patient-e2e-1',
  nhi: 'ABC1234',
  nhiDisplay: 'ABC1234',
  name: 'Jane Test',
  dateOfBirth: '1985-04-12',
  gender: 'female',
  address: '1 Test Street, Wellington',
  phone: '021 555 0100',
  email: 'jane.test@example.nz',
  ethnicity: 'New Zealand European',
  enrolledPractice: 'Test Medical Centre',
  gpName: 'Dr Jane Practitioner',
  allergies: ['Penicillin'],
  alerts: [],
} as const;

export const TEST_PATIENT_LIST = {
  patients: [
    {
      id: TEST_PATIENT.id,
      nhi: TEST_PATIENT.nhi,
      nhiDisplay: TEST_PATIENT.nhiDisplay,
      name: TEST_PATIENT.name,
      dateOfBirth: TEST_PATIENT.dateOfBirth,
      gender: TEST_PATIENT.gender,
      address: TEST_PATIENT.address,
      enrolledPractice: TEST_PATIENT.enrolledPractice,
    },
  ],
  total: 1,
} as const;
