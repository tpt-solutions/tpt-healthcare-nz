package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fetchPatientMobile decrypts the patient's FHIR resource and returns their
// mobile phone in E.164 format, or an error if no mobile is found.
func fetchPatientMobile(ctx context.Context, pool *pgxpool.Pool, enc *encryption.Cipher, patientID uuid.UUID) (string, error) {
	var fhirEnc []byte
	if err := pool.QueryRow(ctx,
		`SELECT fhir_resource FROM patients WHERE id = $1`,
		patientID,
	).Scan(&fhirEnc); err != nil {
		return "", fmt.Errorf("patient mobile lookup: %w", err)
	}

	plain, err := enc.Decrypt(fhirEnc)
	if err != nil {
		return "", fmt.Errorf("patient mobile decrypt: %w", err)
	}

	var p r5.Patient
	if err := json.Unmarshal(plain, &p); err != nil {
		return "", fmt.Errorf("patient mobile unmarshal: %w", err)
	}

	for _, cp := range p.Telecom {
		if cp.System == "phone" && cp.Use == "mobile" && cp.Value != "" {
			return cp.Value, nil
		}
	}
	return "", fmt.Errorf("patient %s has no mobile phone on record", patientID)
}
