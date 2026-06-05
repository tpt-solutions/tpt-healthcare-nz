// apps/tpt-clinic/src/pages/AlliedHealthPage.tsx
import React, { useState, useEffect } from 'react';
import { Box, Grid, Paper, Typography, Tabs, Tab, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Button, TextField, InputAdornment, IconButton, Chip, Dialog, DialogTitle, DialogContent, DialogActions, FormControl, InputLabel, Select, MenuItem, Alert, AlertTitle, Snackbar, CircularProgress, Accordion, AccordionSummary, AccordionDetails, ExpansionPanel, ExpansionPanelSummary, ExpansionPanelDetails, Tooltip, Badge, Avatar, List, ListItem, ListItemText, ListItemSecondaryAction, Divider } from '@mui/material';
import { Add as AddIcon, Edit as EditIcon, Delete as DeleteIcon, Search as SearchIcon, FilterList as FilterIcon, Download as DownloadIcon, Print as PrintIcon, Refresh as RefreshIcon, Person as PersonIcon, MedicalServices as MedicalServicesIcon, Psychology as PsychologyIcon, DirectionsWalk as DirectionsWalkIcon, RecordVoiceOver as RecordVoiceOverIcon, Healing as HealingIcon, LocalHospital as LocalHospitalIcon, Assessment as AssessmentIcon, Description as DescriptionIcon, AttachMoney as AttachMoneyIcon, CalendarToday as CalendarTodayIcon, Warning as WarningIcon, CheckCircle as CheckCircleIcon, Pending as PendingIcon, Error as ErrorIcon, Info as InfoIcon } from '@mui/icons-material';
import { useTheme } from '@mui/material/styles';

interface TreatmentPlan {
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

interface ACCClaim {
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

interface SessionNote {
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

const professionColors = {
  physiotherapy: '#2196F3',
  occupational_therapy: '#4CAF50',
  speech_language_therapy: '#FF9800',
  podiatry: '#9C27B0',
};

const professionIcons: Record<string, React.ReactNode> = {
  physiotherapy: <DirectionsWalkIcon />,
  occupational_therapy: <PsychologyIcon />,
  speech_language_therapy: <RecordVoiceOverIcon />,
  podiatry: <HealingIcon />,
};

const professionLabels = {
  physiotherapy: 'Physiotherapy',
  occupational_therapy: 'Occupational Therapy',
  speech_language_therapy: 'Speech-Language Therapy',
  podiatry: 'Podiatry',
};

const statusColors: Record<string, 'default' | 'primary' | 'secondary' | 'error' | 'info' | 'success' | 'warning'> = {
  draft: 'default',
  active: 'primary',
  under_review: 'warning',
  completed: 'success',
  discontinued: 'error',
  on_hold: 'info',
  submitted: 'info',
  accepted: 'success',
  declined: 'error',
  closed: 'default',
  expired: 'error',
  planned: 'default',
  cancelled: 'error',
};

const mockTreatmentPlans: TreatmentPlan[] = [
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

const mockACCClaims: ACCClaim[] = [
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

const mockSessionNotes: SessionNote[] = [
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

export const AlliedHealthPage: React.FC = () => {
  const theme = useTheme();
  const [activeTab, setActiveTab] = useState(0);
  const [searchTerm, setSearchTerm] = useState('');
  const [professionFilter, setProfessionFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState<'success' | 'error' | 'info' | 'warning'>('info');
  const [loading, setLoading] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState<TreatmentPlan | null>(null);
  const [planDialogOpen, setPlanDialogOpen] = useState(false);

  const filteredPlans = mockTreatmentPlans.filter(plan => {
    const matchesSearch = plan.patientName.toLowerCase().includes(searchTerm.toLowerCase()) ||
      plan.patientNHI.toLowerCase().includes(searchTerm.toLowerCase()) ||
      plan.diagnosis.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesProfession = professionFilter === 'all' || plan.profession === professionFilter;
    const matchesStatus = statusFilter === 'all' || plan.status === statusFilter;
    return matchesSearch && matchesProfession && matchesStatus;
  });

  const filteredClaims = mockACCClaims.filter(claim => {
    const matchesSearch = claim.patientName.toLowerCase().includes(searchTerm.toLowerCase()) ||
      claim.patientNHI.toLowerCase().includes(searchTerm.toLowerCase()) ||
      claim.accNumber.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesProfession = professionFilter === 'all' || claim.claimType === professionFilter;
    const matchesStatus = statusFilter === 'all' || claim.status === statusFilter;
    return matchesSearch && matchesProfession && matchesStatus;
  });

  const filteredSessions = mockSessionNotes.filter(session => {
    const matchesSearch = session.patientName.toLowerCase().includes(searchTerm.toLowerCase()) ||
      session.patientNHI.toLowerCase().includes(searchTerm.toLowerCase()) ||
      session.clinician.toLowerCase().includes(searchTerm.toLowerCase());
    const matchesProfession = professionFilter === 'all' || session.profession === professionFilter;
    const matchesStatus = statusFilter === 'all' || session.status === statusFilter;
    return matchesSearch && matchesProfession && matchesStatus;
  });

  const handleShowSnackbar = (message: string, severity: 'success' | 'error' | 'info' | 'warning' = 'info') => {
    setSnackbarMessage(message);
    setSnackbarSeverity(severity);
    setSnackbarOpen(true);
  };

  const handleViewPlan = (plan: TreatmentPlan) => {
    setSelectedPlan(plan);
    setPlanDialogOpen(true);
  };

  const handleNewPlan = () => {
    handleShowSnackbar('Navigate to profession-specific page to create new treatment plan', 'info');
  };

  const handleNewClaim = () => {
    handleShowSnackbar('Navigate to ACC Claims to create new claim', 'info');
  };

  const handleNewSession = () => {
    handleShowSnackbar('Navigate to profession-specific page to create new session note', 'info');
  };

  const getProfessionColor = (profession: string) => professionColors[profession as keyof typeof professionColors] || '#757575';
  const getProfessionIcon = (profession: string) => professionIcons[profession as keyof typeof professionIcons] || <MedicalServicesIcon />;
  const getProfessionLabel = (profession: string) => professionLabels[profession as keyof typeof professionLabels] || profession;

  const tabs = [
    { label: 'Treatment Plans', icon: <DescriptionIcon />, count: mockTreatmentPlans.length },
    { label: 'ACC Claims', icon: <AttachMoneyIcon />, count: mockACCClaims.length },
    { label: 'Session Notes', icon: <AssessmentIcon />, count: mockSessionNotes.length },
    { label: 'Dashboard', icon: <LocalHospitalIcon />, count: 0 },
  ];

  return (
    <Box sx={{ flexGrow: 1, p: 3, bgcolor: 'background.default', minHeight: '100vh' }}>
      <Box sx={{ mb: 3 }}>
        <Typography variant="h4" component="h1" gutterBottom>
          Allied Health Services
        </Typography>
        <Typography variant="body1" color="text.secondary">
          Physiotherapy, Occupational Therapy, Speech-Language Therapy & Podiatry
        </Typography>
      </Box>

      {/* Stats Cards */}
      <Grid container spacing={3} sx={{ mb: 3 }}>
        <Grid item xs={12} sm={6} md={3}>
          <Paper elevation={2} sx={{ p: 2, textAlign: 'center', borderLeft: `4px solid ${professionColors.physiotherapy}` }}>
            <Typography variant="h5" color={professionColors.physiotherapy}>
              {mockTreatmentPlans.filter(p => p.profession === 'physiotherapy').length}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Physiotherapy Plans
            </Typography>
          </Paper>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Paper elevation={2} sx={{ p: 2, textAlign: 'center', borderLeft: `4px solid ${professionColors.occupational_therapy}` }}>
            <Typography variant="h5" color={professionColors.occupational_therapy}>
              {mockTreatmentPlans.filter(p => p.profession === 'occupational_therapy').length}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              OT Plans
            </Typography>
          </Paper>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Paper elevation={2} sx={{ p: 2, textAlign: 'center', borderLeft: `4px solid ${professionColors.speech_language_therapy}` }}>
            <Typography variant="h5" color={professionColors.speech_language_therapy}>
              {mockTreatmentPlans.filter(p => p.profession === 'speech_language_therapy').length}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Speech Therapy Plans
            </Typography>
          </Paper>
        </Grid>
        <Grid item xs={12} sm={6} md={3}>
          <Paper elevation={2} sx={{ p: 2, textAlign: 'center', borderLeft: `4px solid ${professionColors.podiatry}` }}>
            <Typography variant="h5" color={professionColors.podiatry}>
              {mockTreatmentPlans.filter(p => p.profession === 'podiatry').length}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Podiatry Plans
            </Typography>
          </Paper>
        </Grid>
      </Grid>

      {/* Filters */}
      <Paper elevation={1} sx={{ p: 2, mb: 3 }}>
        <Grid container spacing={2} alignItems="flex-end">
          <Grid item xs={12} sm={4}>
            <TextField
              fullWidth
              placeholder="Search patients, NHI, diagnosis..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon color="action" />
                  </InputAdornment>
                ),
              }}
              size="small"
            />
          </Grid>
          <Grid item xs={12} sm={3}>
            <FormControl fullWidth size="small">
              <InputLabel>Profession</InputLabel>
              <Select
                value={professionFilter}
                label="Profession"
                onChange={(e) => setProfessionFilter(e.target.value)}
              >
                <MenuItem value="all">All Professions</MenuItem>
                <MenuItem value="physiotherapy">Physiotherapy</MenuItem>
                <MenuItem value="occupational_therapy">Occupational Therapy</MenuItem>
                <MenuItem value="speech_language_therapy">Speech-Language Therapy</MenuItem>
                <MenuItem value="podiatry">Podiatry</MenuItem>
              </Select>
            </FormControl>
          </Grid>
          <Grid item xs={12} sm={3}>
            <FormControl fullWidth size="small">
              <InputLabel>Status</InputLabel>
              <Select
                value={statusFilter}
                label="Status"
                onChange={(e) => setStatusFilter(e.target.value)}
              >
                <MenuItem value="all">All Statuses</MenuItem>
                <MenuItem value="draft">Draft</MenuItem>
                <MenuItem value="active">Active</MenuItem>
                <MenuItem value="under_review">Under Review</MenuItem>
                <MenuItem value="completed">Completed</MenuItem>
                <MenuItem value="discontinued">Discontinued</MenuItem>
                <MenuItem value="on_hold">On Hold</MenuItem>
                <MenuItem value="submitted">Submitted</MenuItem>
                <MenuItem value="accepted">Accepted</MenuItem>
                <MenuItem value="declined">Declined</MenuItem>
                <MenuItem value="closed">Closed</MenuItem>
                <MenuItem value="expired">Expired</MenuItem>
                <MenuItem value="planned">Planned</MenuItem>
                <MenuItem value="cancelled">Cancelled</MenuItem>
              </Select>
            </FormControl>
          </Grid>
          <Grid item xs={12} sm={2}>
            <Button
              variant="outlined"
              fullWidth
              startIcon={<RefreshIcon />}
              onClick={() => handleShowSnackbar('Data refreshed', 'success')}
              size="small"
            >
              Refresh
            </Button>
          </Grid>
        </Grid>
      </Paper>

      {/* Tabs */}
      <Paper elevation={2} sx={{ mb: 2 }}>
        <Tabs
          value={activeTab}
          onChange={(_, value) => setActiveTab(value)}
          variant="scrollable"
          scrollButtons="auto"
          indicatorColor="primary"
          textColor="primary"
          sx={{ borderBottom: 1, borderColor: 'divider' }}
        >
          {tabs.map((tab, index) => (
            <Tab
              key={index}
              icon={tab.icon}
              label={`${tab.label} (${tab.count})`}
              sx={{ minWidth: 160, textTransform: 'none', fontWeight: 500 }}
            />
          ))}
        </Tabs>
      </Paper>

      {/* Tab Panels */}
      {activeTab === 0 && (
        // Treatment Plans Tab
        <Paper elevation={2}>
          <Box sx={{ p: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="h6">Treatment Plans</Typography>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={handleNewPlan}
              sx={{ ml: 1 }}
            >
              New Plan
            </Button>
          </Box>
          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Patient</TableCell>
                  <TableCell>Profession</TableCell>
                  <TableCell>Clinician</TableCell>
                  <TableCell>Diagnosis</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>ACC Claim</TableCell>
                  <TableCell>Sessions</TableCell>
                  <TableCell>Review Date</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredPlans.map((plan) => (
                  <TableRow key={plan.id} hover onClick={() => handleViewPlan(plan)} style={{ cursor: 'pointer' }}>
                    <TableCell>
                      <Box>
                        <Typography variant="body2" fontWeight={500}>{plan.patientName}</Typography>
                        <Typography variant="caption" color="text.secondary">NHI: {plan.patientNHI}</Typography>
                      </Box>
                    </TableCell>
                    <TableCell>
                      <Chip
                        icon={getProfessionIcon(plan.profession)}
                        label={getProfessionLabel(plan.profession)}
                        size="small"
                        color="primary"
                        variant="outlined"
                        sx={{ backgroundColor: `${getProfessionColor(plan.profession)}10`, borderColor: getProfessionColor(plan.profession) }}
                      />
                    </TableCell>
                    <TableCell>{plan.clinician}</TableCell>
                    <TableCell>
                      <Typography variant="body2" maxWidth={200} noWrap textOverflow="ellipsis" overflow="hidden">
                        {plan.diagnosis}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={plan.status.replace('_', ' ').toUpperCase()}
                        size="small"
                        color={statusColors[plan.status]}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell>{plan.accNumber || '-'}</TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {plan.sessionsUsed} / {plan.sessionsApproved}
                      </Typography>
                      <Box sx={{ mt: 0.5 }}>
                        <Box
                          sx={{
                            height: 4,
                            borderRadius: 2,
                            bgcolor: 'grey.200',
                            overflow: 'hidden',
                          }}
                        >
                          <Box
                            sx={{
                              height: '100%',
                              width: `${(plan.sessionsUsed / plan.sessionsApproved) * 100}%`,
                              bgcolor: plan.sessionsUsed >= plan.sessionsApproved ? 'error.main' : getProfessionColor(plan.profession),
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </Box>
                      </Box>
                    </TableCell>
                    <TableCell>{plan.reviewDate}</TableCell>
                    <TableCell align="right">
                      <IconButton size="small" onClick={(e) => { e.stopPropagation(); handleViewPlan(plan); }} aria-label="View">
                        <EditIcon fontSize="small" />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
                {filteredPlans.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={9} align="center" sx={{ py: 4 }}>
                      <Typography color="text.secondary">No treatment plans found</Typography>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}

      {activeTab === 1 && (
        // ACC Claims Tab
        <Paper elevation={2}>
          <Box sx={{ p: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="h6">ACC Claims</Typography>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={handleNewClaim}
              sx={{ ml: 1 }}
            >
              New Claim
            </Button>
          </Box>
          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Patient</TableCell>
                  <TableCell>Claim Type</TableCell>
                  <TableCell>ACC Number</TableCell>
                  <TableCell>Diagnosis</TableCell>
                  <TableCell>Body Region</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Sessions</TableCell>
                  <TableCell>Expiry Date</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredClaims.map((claim) => (
                  <TableRow key={claim.id} hover>
                    <TableCell>
                      <Box>
                        <Typography variant="body2" fontWeight={500}>{claim.patientName}</Typography>
                        <Typography variant="caption" color="text.secondary">NHI: {claim.patientNHI}</Typography>
                      </Box>
                    </TableCell>
                    <TableCell>
                      <Chip
                        icon={getProfessionIcon(claim.claimType)}
                        label={getProfessionLabel(claim.claimType)}
                        size="small"
                        color="primary"
                        variant="outlined"
                        sx={{ backgroundColor: `${getProfessionColor(claim.claimType)}10`, borderColor: getProfessionColor(claim.claimType) }}
                      />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" fontFamily="monospace" fontWeight={500}>
                        {claim.accNumber}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" maxWidth={200} noWrap textOverflow="ellipsis" overflow="hidden">
                        {claim.diagnosis}
                      </Typography>
                    </TableCell>
                    <TableCell>{claim.bodyRegion.replace('_', ' ')}</TableCell>
                    <TableCell>
                      <Chip
                        label={claim.status.replace('_', ' ').toUpperCase()}
                        size="small"
                        color={statusColors[claim.status]}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {claim.usedSessions} / {claim.approvedSessions}
                      </Typography>
                      <Box sx={{ mt: 0.5 }}>
                        <Box
                          sx={{
                            height: 4,
                            borderRadius: 2,
                            bgcolor: 'grey.200',
                            overflow: 'hidden',
                          }}
                        >
                          <Box
                            sx={{
                              height: '100%',
                              width: `${(claim.usedSessions / claim.approvedSessions) * 100}%`,
                              bgcolor: claim.usedSessions >= claim.approvedSessions ? 'error.main' : getProfessionColor(claim.claimType),
                              transition: 'width 0.3s ease',
                            }}
                          />
                        </Box>
                      </Box>
                    </TableCell>
                    <TableCell>{claim.expiryDate}</TableCell>
                    <TableCell align="right">
                      <IconButton size="small" aria-label="View claim">
                        <EditIcon fontSize="small" />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
                {filteredClaims.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={9} align="center" sx={{ py: 4 }}>
                      <Typography color="text.secondary">No ACC claims found</Typography>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}

      {activeTab === 2 && (
        // Session Notes Tab
        <Paper elevation={2}>
          <Box sx={{ p: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: 1, borderColor: 'divider' }}>
            <Typography variant="h6">Session Notes</Typography>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={handleNewSession}
              sx={{ ml: 1 }}
            >
              New Session
            </Button>
          </Box>
          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Patient</TableCell>
                  <TableCell>Profession</TableCell>
                  <TableCell>Clinician</TableCell>
                  <TableCell>Session Date</TableCell>
                  <TableCell>Session #</TableCell>
                  <TableCell>Duration</TableCell>
                  <TableCell>Charge Code</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {filteredSessions.map((session) => (
                  <TableRow key={session.id} hover>
                    <TableCell>
                      <Box>
                        <Typography variant="body2" fontWeight={500}>{session.patientName}</Typography>
                        <Typography variant="caption" color="text.secondary">NHI: {session.patientNHI}</Typography>
                      </Box>
                    </TableCell>
                    <TableCell>
                      <Chip
                        icon={getProfessionIcon(session.profession)}
                        label={getProfessionLabel(session.profession)}
                        size="small"
                        color="primary"
                        variant="outlined"
                        sx={{ backgroundColor: `${getProfessionColor(session.profession)}10`, borderColor: getProfessionColor(session.profession) }}
                      />
                    </TableCell>
                    <TableCell>{session.clinician}</TableCell>
                    <TableCell>{session.sessionDate}</TableCell>
                    <TableCell>{session.sessionNumber}</TableCell>
                    <TableCell>{session.durationMinutes} min</TableCell>
                    <TableCell>
                      <Typography variant="body2" fontFamily="monospace">{session.chargeCode}</Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={session.status.toUpperCase()}
                        size="small"
                        color={statusColors[session.status]}
                        variant="outlined"
                      />
                    </TableCell>
                    <TableCell align="right">
                      <IconButton size="small" aria-label="View session">
                        <EditIcon fontSize="small" />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
                {filteredSessions.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={9} align="center" sx={{ py: 4 }}>
                      <Typography color="text.secondary">No session notes found</Typography>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      )}

      {activeTab === 3 && (
        // Dashboard Tab
        <Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <Paper elevation={2} sx={{ p: 3 }}>
              <Typography variant="h6" gutterBottom>Profession Distribution</Typography>
              <Grid container spacing={2}>
                {Object.entries(professionLabels).map(([key, label]) => (
                  <Grid item xs={6} key={key}>
                    <Box sx={{ p: 2, textAlign: 'center', bgcolor: `${professionColors[key as keyof typeof professionColors]}10`, borderRadius: 2, border: `1px solid ${professionColors[key as keyof typeof professionColors]}40` }}>
                      {getProfessionIcon(key)}
                      <Typography variant="h6" color={getProfessionColor(key)}>
                        {mockTreatmentPlans.filter(p => p.profession === key as any).length}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">{label}</Typography>
                    </Box>
                  </Grid>
                ))}
              </Grid>
            </Paper>
          </Grid>
          <Grid item xs={12} md={6}>
            <Paper elevation={2} sx={{ p: 3 }}>
              <Typography variant="h6" gutterBottom>Claim Status Overview</Typography>
              <Grid container spacing={2}>
                {['accepted', 'under_review', 'submitted', 'draft'].map(status => (
                  <Grid item xs={6} key={status}>
                    <Box sx={{ p: 2, textAlign: 'center', bgcolor: `${theme.palette[statusColors[status] as keyof typeof theme.palette]?.main || theme.palette.grey[500]}10`, borderRadius: 2 }}>
                      <Typography variant="h6" color={theme.palette[statusColors[status] as keyof typeof theme.palette]?.main || theme.palette.grey[500]}>
                        {mockACCClaims.filter(c => c.status === status).length}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">{status.replace('_', ' ').toUpperCase()}</Typography>
                    </Box>
                  </Grid>
                ))}
              </Grid>
            </Paper>
          </Grid>
          <Grid item xs={12} md={6}>
            <Paper elevation={2} sx={{ p: 3 }}>
              <Typography variant="h6" gutterBottom>Upcoming Reviews</Typography>
              <List>
                {mockTreatmentPlans
                  .filter(p => p.status === 'active' || p.status === 'under_review')
                  .sort((a, b) => new Date(a.reviewDate).getTime() - new Date(b.reviewDate).getTime())
                  .slice(0, 5)
                  .map(plan => (
                    <ListItem key={plan.id} divider>
                      <ListItemText
                        primary={plan.patientName}
                        secondary={`${getProfessionLabel(plan.profession)} • Review: ${plan.reviewDate} • ${plan.clinician}`}
                      />
                      <ListItemSecondaryAction>
                        <Chip label={plan.status} size="small" color={statusColors[plan.status]} variant="outlined" />
                      </ListItemSecondaryAction>
                    </ListItem>
                  ))}
              </List>
            </Paper>
          </Grid>
          <Grid item xs={12} md={6}>
            <Paper elevation={2} sx={{ p: 3 }}>
              <Typography variant="h6" gutterBottom>Expiring Claims</Typography>
              <List>
                {mockACCClaims
                  .filter(c => c.status === 'accepted')
                  .sort((a, b) => new Date(a.expiryDate).getTime() - new Date(b.expiryDate).getTime())
                  .slice(0, 5)
                  .map(claim => (
                    <ListItem key={claim.id} divider>
                      <ListItemText
                        primary={claim.patientName}
                        secondary={`${getProfessionLabel(claim.claimType)} • Expires: ${claim.expiryDate} • ${claim.usedSessions}/${claim.approvedSessions} sessions used`}
                      />
                      <ListItemSecondaryAction>
                        <Chip label={claim.status} size="small" color={statusColors[claim.status]} variant="outlined" />
                      </ListItemSecondaryAction>
                    </ListItem>
                  ))}
              </List>
            </Paper>
          </Grid>
        </Grid>
      )}

      {/* Plan Detail Dialog */}
      <Dialog open={planDialogOpen} onClose={() => setPlanDialogOpen(false)} maxWidth="md" fullWidth>
        {selectedPlan && (
          <>
            <DialogTitle>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                <Chip
                  icon={getProfessionIcon(selectedPlan.profession)}
                  label={getProfessionLabel(selectedPlan.profession)}
                  size="small"
                  color="primary"
                  variant="outlined"
                  sx={{ backgroundColor: `${getProfessionColor(selectedPlan.profession)}10`, borderColor: getProfessionColor(selectedPlan.profession) }}
                />
                <Typography variant="h6">{selectedPlan.patientName}</Typography>
              </Box>
            </DialogTitle>
            <DialogContent>
              <Grid container spacing={2} sx={{ p: 1 }}>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">NHI</Typography>
                  <Typography variant="body1" fontWeight={500}>{selectedPlan.patientNHI}</Typography>
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">Clinician</Typography>
                  <Typography variant="body1">{selectedPlan.clinician}</Typography>
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">Diagnosis</Typography>
                  <Typography variant="body1">{selectedPlan.diagnosis}</Typography>
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">ACC Claim</Typography>
                  <Typography variant="body1" fontFamily="monospace">{selectedPlan.accNumber || 'N/A'}</Typography>
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">Status</Typography>
                  <Chip label={selectedPlan.status.replace('_', ' ').toUpperCase()} color={statusColors[selectedPlan.status]} variant="outlined" />
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">Sessions</Typography>
                  <Typography variant="body1">{selectedPlan.sessionsUsed} / {selectedPlan.sessionsApproved}</Typography>
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">Start Date</Typography>
                  <Typography variant="body1">{selectedPlan.startDate}</Typography>
                </Grid>
                <Grid item xs={12} sm={6}>
                  <Typography variant="body2" color="text.secondary">Review Date</Typography>
                  <Typography variant="body1">{selectedPlan.reviewDate}</Typography>
                </Grid>
                <Grid item xs={12}>
                  <Divider sx={{ my: 2 }} />
                  <Typography variant="subtitle1" gutterBottom>Goals & Interventions</Typography>
                  <Typography variant="body2" color="text.secondary">
                    View detailed goals, interventions, and outcome measures in the profession-specific module.
                  </Typography>
                </Grid>
              </Grid>
            </DialogContent>
            <DialogActions>
              <Button onClick={() => setPlanDialogOpen(false)}>Close</Button>
              <Button variant="contained" startIcon={<EditIcon />}>Edit Plan</Button>
            </DialogActions>
          </>
        )}
      </Dialog>

      {/* Snackbar */}
      <Snackbar
        open={snackbarOpen}
        autoHideDuration={6000}
        onClose={() => setSnackbarOpen(false)}
      >
        <Alert
          severity={snackbarSeverity}
          onClose={() => setSnackbarOpen(false)}
          sx={{ width: '100%' }}
        >
          {snackbarMessage}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default AlliedHealthPage;