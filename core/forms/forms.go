// Package forms provides a FHIR Questionnaire-backed intake form engine.
// Forms are defined as FHIR R5 Questionnaire resources stored in the FHIR repo.
// Patient responses are stored as QuestionnaireResponse resources and can be
// used to pre-populate encounter notes, update patient demographics, and
// populate allergy/medication lists before a consultation.
//
// Workflow:
//  1. Clinician or admin creates a Questionnaire template linked to an appointment type.
//  2. On appointment confirmation, a FormInstance is created and a link is emailed/pushed
//     to the patient via the patient portal.
//  3. Patient completes the form in the portal; a QuestionnaireResponse is stored.
//  4. On the clinician's encounter screen, the pre-filled response is shown as a sidebar.
package forms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QuestionType describes the type of a single form question.
type QuestionType string

const (
	TypeText     QuestionType = "text"
	TypeTextArea QuestionType = "textarea"
	TypeBoolean  QuestionType = "boolean"
	TypeChoice   QuestionType = "choice"    // single-select from options
	TypeMulti    QuestionType = "multi"     // multi-select
	TypeDate     QuestionType = "date"
	TypeNumber   QuestionType = "decimal"
)

// FormQuestion is a single item in a form template.
type FormQuestion struct {
	// LinkID is a stable identifier used to correlate question → answer.
	LinkID  string       `json:"linkId"`
	Text    string       `json:"text"`
	Type    QuestionType `json:"type"`
	Required bool        `json:"required"`
	// Options holds the selectable values for choice/multi questions.
	Options []string     `json:"options,omitempty"`
	// HelpText is shown below the question field.
	HelpText string      `json:"helpText,omitempty"`
}

// FormTemplate defines a reusable questionnaire linked to appointment types.
type FormTemplate struct {
	ID             uuid.UUID      `json:"id"`
	TenantID       uuid.UUID      `json:"tenantId"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	// AppointmentTypes is a list of appointment type slugs this form is sent for.
	// An empty list means the form must be triggered manually.
	AppointmentTypes []string     `json:"appointmentTypes,omitempty"`
	Questions      []FormQuestion `json:"questions"`
	Active         bool           `json:"active"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// FormInstanceStatus tracks where the patient is in the form lifecycle.
type FormInstanceStatus string

const (
	InstancePending   FormInstanceStatus = "pending"
	InstanceSent      FormInstanceStatus = "sent"
	InstanceCompleted FormInstanceStatus = "completed"
	InstanceExpired   FormInstanceStatus = "expired"
)

// FormInstance is a single use of a FormTemplate for a specific patient+appointment.
type FormInstance struct {
	ID             uuid.UUID          `json:"id"`
	TenantID       uuid.UUID          `json:"tenantId"`
	TemplateID     uuid.UUID          `json:"templateId"`
	PatientID      string             `json:"patientId"`
	AppointmentID  string             `json:"appointmentId,omitempty"`
	Status         FormInstanceStatus `json:"status"`
	// Token is a short-lived URL-safe token embedded in the patient portal link.
	Token          string             `json:"token"`
	// ExpiresAt is when the form link expires; typically appointment start time.
	ExpiresAt      time.Time          `json:"expiresAt"`
	// CompletedAt is when the patient submitted the form.
	CompletedAt    *time.Time         `json:"completedAt,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
}

// FormAnswer is a patient's answer to a single question.
type FormAnswer struct {
	LinkID string          `json:"linkId"`
	Value  json.RawMessage `json:"value"` // string, bool, []string, or number depending on type
}

// FormResponse is the patient's submitted response to a FormInstance.
type FormResponse struct {
	ID         uuid.UUID    `json:"id"`
	InstanceID uuid.UUID    `json:"instanceId"`
	TenantID   uuid.UUID    `json:"tenantId"`
	PatientID  string       `json:"patientId"`
	Answers    []FormAnswer `json:"answers"`
	SubmittedAt time.Time   `json:"submittedAt"`
}

// Service manages form templates, instances, and responses.
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a forms Service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateTemplate persists a new FormTemplate.
func (s *Service) CreateTemplate(ctx context.Context, t FormTemplate) (*FormTemplate, error) {
	t.ID = uuid.New()
	t.Active = true
	t.CreatedAt = time.Now().UTC()
	t.UpdatedAt = t.CreatedAt

	qJSON, err := json.Marshal(t.Questions)
	if err != nil {
		return nil, fmt.Errorf("forms: marshaling questions: %w", err)
	}
	apptJSON, err := json.Marshal(t.AppointmentTypes)
	if err != nil {
		return nil, fmt.Errorf("forms: marshaling appointment types: %w", err)
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO form_templates
			(id, tenant_id, title, description, appointment_types, questions, active, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		t.ID, t.TenantID, t.Title, t.Description, apptJSON, qJSON, t.Active, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("forms: creating template: %w", err)
	}
	return &t, nil
}

// GetTemplate retrieves a FormTemplate by ID.
func (s *Service) GetTemplate(ctx context.Context, tenantID, templateID uuid.UUID) (*FormTemplate, error) {
	var t FormTemplate
	var questionsJSON, apptTypesJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, title, description, appointment_types, questions, active, created_at, updated_at
		 FROM form_templates WHERE id=$1 AND tenant_id=$2`,
		templateID, tenantID,
	).Scan(&t.ID, &t.TenantID, &t.Title, &t.Description, &apptTypesJSON, &questionsJSON,
		&t.Active, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("forms: getting template %s: %w", templateID, err)
	}
	_ = json.Unmarshal(questionsJSON, &t.Questions)
	_ = json.Unmarshal(apptTypesJSON, &t.AppointmentTypes)
	return &t, nil
}

// CreateInstance generates a FormInstance for a patient+appointment.
// A URL-safe token is generated for the patient portal link.
func (s *Service) CreateInstance(ctx context.Context, tenantID, templateID uuid.UUID, patientID, appointmentID string, expiresAt time.Time) (*FormInstance, error) {
	inst := &FormInstance{
		ID:            uuid.New(),
		TenantID:      tenantID,
		TemplateID:    templateID,
		PatientID:     patientID,
		AppointmentID: appointmentID,
		Status:        InstancePending,
		Token:         generateToken(),
		ExpiresAt:     expiresAt,
		CreatedAt:     time.Now().UTC(),
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO form_instances
			(id, tenant_id, template_id, patient_id, appointment_id, status, token, expires_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		inst.ID, inst.TenantID, inst.TemplateID, inst.PatientID,
		nilIfEmpty(inst.AppointmentID), inst.Status, inst.Token, inst.ExpiresAt, inst.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("forms: creating instance: %w", err)
	}
	return inst, nil
}

// GetInstanceByToken retrieves a FormInstance by its patient portal token.
// Returns an error if the token is expired or not found.
func (s *Service) GetInstanceByToken(ctx context.Context, token string) (*FormInstance, error) {
	var inst FormInstance
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, template_id, patient_id,
			COALESCE(appointment_id,''), status, token, expires_at, completed_at, created_at
		 FROM form_instances WHERE token=$1`,
		token,
	).Scan(&inst.ID, &inst.TenantID, &inst.TemplateID, &inst.PatientID,
		&inst.AppointmentID, &inst.Status, &inst.Token, &inst.ExpiresAt, &inst.CompletedAt, &inst.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("forms: token not found: %w", err)
	}
	if time.Now().UTC().After(inst.ExpiresAt) {
		return nil, fmt.Errorf("forms: form link has expired")
	}
	return &inst, nil
}

// SubmitResponse records the patient's answers and marks the instance completed.
func (s *Service) SubmitResponse(ctx context.Context, instanceID, tenantID uuid.UUID, patientID string, answers []FormAnswer) (*FormResponse, error) {
	answersJSON, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("forms: marshaling answers: %w", err)
	}

	resp := &FormResponse{
		ID:          uuid.New(),
		InstanceID:  instanceID,
		TenantID:    tenantID,
		PatientID:   patientID,
		Answers:     answers,
		SubmittedAt: time.Now().UTC(),
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("forms: beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = tx.Exec(ctx,
		`INSERT INTO form_responses (id, instance_id, tenant_id, patient_id, answers, submitted_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		resp.ID, resp.InstanceID, resp.TenantID, resp.PatientID, answersJSON, resp.SubmittedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("forms: inserting response: %w", err)
	}

	_, err = tx.Exec(ctx,
		`UPDATE form_instances SET status='completed', completed_at=$1 WHERE id=$2`,
		resp.SubmittedAt, instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("forms: marking instance complete: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("forms: committing: %w", err)
	}
	return resp, nil
}

// GetResponseForInstance retrieves the submitted response for a form instance.
func (s *Service) GetResponseForInstance(ctx context.Context, tenantID, instanceID uuid.UUID) (*FormResponse, error) {
	var resp FormResponse
	var answersJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT id, instance_id, tenant_id, patient_id, answers, submitted_at
		 FROM form_responses WHERE instance_id=$1 AND tenant_id=$2`,
		instanceID, tenantID,
	).Scan(&resp.ID, &resp.InstanceID, &resp.TenantID, &resp.PatientID, &answersJSON, &resp.SubmittedAt)
	if err != nil {
		return nil, fmt.Errorf("forms: getting response for instance %s: %w", instanceID, err)
	}
	_ = json.Unmarshal(answersJSON, &resp.Answers)
	return &resp, nil
}

// ListTemplatesForAppointmentType returns all active templates that should be
// sent for a given appointment type slug.
func (s *Service) ListTemplatesForAppointmentType(ctx context.Context, tenantID uuid.UUID, apptType string) ([]FormTemplate, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, title, description, appointment_types, questions, active, created_at, updated_at
		 FROM form_templates
		 WHERE tenant_id=$1 AND active=true
		   AND appointment_types @> $2::jsonb`,
		tenantID, fmt.Sprintf("[%q]", apptType),
	)
	if err != nil {
		return nil, fmt.Errorf("forms: listing templates for appt type %s: %w", apptType, err)
	}
	defer rows.Close()

	var templates []FormTemplate
	for rows.Next() {
		var t FormTemplate
		var qJSON, atJSON []byte
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Title, &t.Description, &atJSON, &qJSON,
			&t.Active, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(qJSON, &t.Questions)
		_ = json.Unmarshal(atJSON, &t.AppointmentTypes)
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func generateToken() string {
	return uuid.New().String()
}
