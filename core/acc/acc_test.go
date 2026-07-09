package acc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCapFor(t *testing.T) {
	tests := []struct {
		discipline Discipline
		wantCap    int
		wantErr    bool
	}{
		{DisciplineAcupuncture, 16, false},
		{DisciplineChiropractic, 16, false},
		{DisciplineMassage, 16, false},
		{DisciplinePhysiotherapy, 16, false},
		{DisciplineOsteopathy, 16, false},
		{DisciplineCounselling, 0, false},
		{"unknown", 0, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.discipline), func(t *testing.T) {
			cap, err := SessionCapFor(tt.discipline)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantCap, cap.InitialGranted)
			}
		})
	}
}

func TestCodesFor(t *testing.T) {
	tests := []struct {
		discipline Discipline
		wantErr    bool
	}{
		{DisciplineAcupuncture, false},
		{DisciplineChiropractic, false},
		{DisciplineMassage, false},
		{DisciplinePhysiotherapy, false},
		{DisciplineOsteopathy, false},
		{DisciplineCounselling, false},
		{"unknown", true},
	}
	for _, tt := range tests {
		t.Run(string(tt.discipline), func(t *testing.T) {
			codes, err := CodesFor(tt.discipline)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, codes, 2)
			}
		})
	}
}

func TestProcedureCodeFor(t *testing.T) {
	t.Run("initial", func(t *testing.T) {
		code, err := ProcedureCodeFor(DisciplineAcupuncture, true)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
	})
	t.Run("subsequent", func(t *testing.T) {
		code, err := ProcedureCodeFor(DisciplineAcupuncture, false)
		require.NoError(t, err)
		assert.NotEmpty(t, code)
	})
	t.Run("unknown", func(t *testing.T) {
		_, err := ProcedureCodeFor("unknown", true)
		assert.Error(t, err)
	})
}

func TestACCError_Error(t *testing.T) {
	t.Run("with claim number", func(t *testing.T) {
		err := &ACCError{Code: "E001", Message: "test error", ClaimNumber: "CLM123"}
		s := err.Error()
		assert.Contains(t, s, "E001")
		assert.Contains(t, s, "test error")
		assert.Contains(t, s, "CLM123")
	})
	t.Run("without claim number", func(t *testing.T) {
		err := &ACCError{Code: "E002", Message: "another error"}
		s := err.Error()
		assert.Contains(t, s, "E002")
	})
}

func TestOutcomeToStatus(t *testing.T) {
	tests := []struct {
		outcome string
		status  ClaimStatus
	}{
		{"complete", ClaimComplete},
		{"error", ClaimDeclined},
		{"partial", ClaimDeclined},
		{"queued", ClaimPending},
		{"active", ClaimActive},
		{"registered", ClaimActive},
		{"unknown", ClaimActive},
	}
	for _, tt := range tests {
		t.Run(tt.outcome, func(t *testing.T) {
			assert.Equal(t, tt.status, outcomeToStatus(tt.outcome))
		})
	}
}

func TestBuildFHIRClaim(t *testing.T) {
	claim := Claim{
		ID:                uuid.New(),
		PatientNHI:        "ZAC1234",
		ProviderHPI:       "HPI5678",
		DateOfAccident:    time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		InjuryDescription: "Lower back strain",
		DiagnosisCodes:    []string{"S39.012"},
	}
	result := buildFHIRClaim(claim)
	assert.Equal(t, "Claim", result["resourceType"])
	assert.NotNil(t, result["patient"])
	assert.NotNil(t, result["provider"])
}

func TestBuildPORequestTask(t *testing.T) {
	req := PORequest{
		ClaimNumber:  "CLM123",
		PatientNHI:   "ZAC1234",
		ProviderHPI:  "HPI5678",
		Discipline:   DisciplinePhysiotherapy,
	}
	result := buildPORequestTask(req)
	assert.Equal(t, "Task", result["resourceType"])
}

func TestDecodeClaimResponse(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		body := map[string]any{
			"resourceType": "Claim",
			"id":           "claim-1",
			"status":       "active",
			"outcome":      "registered",
		}
		data, _ := json.Marshal(body)
		reader := bytes.NewReader(data)
		base := Claim{PatientNHI: "ZAC1234"}
		claim, err := decodeClaimResponse(reader, base)
		require.NoError(t, err)
		assert.Equal(t, "ZAC1234", claim.PatientNHI)
	})
	t.Run("invalid JSON", func(t *testing.T) {
		reader := bytes.NewReader([]byte("not-json"))
		_, err := decodeClaimResponse(reader, Claim{})
		assert.Error(t, err)
	})
}

func TestParseErrorResponse(t *testing.T) {
	t.Run("operation outcome", func(t *testing.T) {
		outcome := map[string]any{
			"resourceType": "OperationOutcome",
			"issue": []any{
				map[string]any{
					"severity":     "error",
					"code":         "invalid",
					"diagnostics": "Invalid claim",
				},
			},
		}
		data, _ := json.Marshal(outcome)
		resp := &http.Response{
			StatusCode: 400,
			Body:       io.NopCloser(bytes.NewReader(data)),
		}
		err := parseErrorResponse(resp, "CLM123")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CLM123")
	})
	t.Run("non-json body", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(bytes.NewReader([]byte("internal error"))),
		}
		err := parseErrorResponse(resp, "CLM123")
		assert.Error(t, err)
	})
}

func TestSetHeaders(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://test.com", nil)
	setHeaders(req, "test-token-123")
	assert.Equal(t, "Bearer test-token-123", req.Header.Get("Authorization"))
	assert.Equal(t, "application/fhir+json", req.Header.Get("Content-Type"))
}

func TestPurchaseOrder_SessionsRemaining(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		po := &PurchaseOrder{SessionsApproved: 12, SessionsUsed: 5}
		assert.Equal(t, 7, po.SessionsRemaining())
	})
	t.Run("zero used", func(t *testing.T) {
		po := &PurchaseOrder{SessionsApproved: 12, SessionsUsed: 0}
		assert.Equal(t, 12, po.SessionsRemaining())
	})
	t.Run("all used", func(t *testing.T) {
		po := &PurchaseOrder{SessionsApproved: 12, SessionsUsed: 12}
		assert.Equal(t, 0, po.SessionsRemaining())
	})
	t.Run("over-used clamps to 0", func(t *testing.T) {
		po := &PurchaseOrder{SessionsApproved: 12, SessionsUsed: 15}
		assert.Equal(t, 0, po.SessionsRemaining())
	})
}

func TestReconcilePOs(t *testing.T) {
	pos := []PurchaseOrder{
		{
			PONumber:        "PO001",
			ClaimNumber:     "CLM001",
			Discipline:      DisciplinePhysiotherapy,
			SessionsApproved: 24,
			SessionsUsed:    10,
			Status:          POApproved,
		},
		{
			PONumber:        "PO002",
			ClaimNumber:     "CLM001",
			Discipline:      DisciplinePhysiotherapy,
			SessionsApproved: 12,
			SessionsUsed:    12,
			Status:          POExhausted,
		},
	}
	lines := ReconcilePOs(pos)
	assert.Len(t, lines, 2)
	assert.Equal(t, "PO001", lines[0].PONumber)
	assert.Equal(t, 14, lines[0].SessionsRemaining)
	assert.Equal(t, 0, lines[1].SessionsRemaining)
}

func TestConsumeSession_PreFlightChecks(t *testing.T) {
	t.Run("non-approved status returns error", func(t *testing.T) {
		po := &PurchaseOrder{PONumber: "PO001", Status: POPending, SessionsApproved: 12, SessionsUsed: 5}
		client := New("http://test.com", func(_ context.Context) (string, error) { return "token", nil })
		_, err := client.ConsumeSession(t.Context(), po)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot consume session")
	})

	t.Run("exhausted sessions returns POExhausted without HTTP", func(t *testing.T) {
		po := &PurchaseOrder{PONumber: "PO001", Status: POApproved, SessionsApproved: 12, SessionsUsed: 12}
		client := New("http://test.com", func(_ context.Context) (string, error) { return "token", nil })
		result, err := client.ConsumeSession(t.Context(), po)
		require.NoError(t, err)
		assert.Equal(t, POExhausted, result.Status)
	})
}

func TestNewClient(t *testing.T) {
	t.Run("trailing slash trimmed", func(t *testing.T) {
		c := New("http://test.com/", func(_ context.Context) (string, error) { return "tok", nil })
		assert.Equal(t, "http://test.com", c.baseURL)
	})
}
