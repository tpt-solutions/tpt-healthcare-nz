// Package private provides private practice management for counselling,
// including encrypted PHI storage for client records.
package private

import (
	"fmt"

	"github.com/PhillipC05/tpt-healthcare/core/encryption"
)

// PrivateClientRequest is the decoded request body when creating/updating a client.
// All PHI fields are plaintext here — they must be encrypted before persistence.
type PrivateClientRequest struct {
	NHI      string `json:"nhi"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Employer string `json:"employer,omitempty"`
	Notes    string `json:"notes,omitempty"`
	Active   bool   `json:"active"`
}

// PrivateClient stores a counselling client with PHI fields encrypted at rest.
// Encrypted fields hold the AES-256-GCM ciphertext produced by core/encryption.
type PrivateClient struct {
	ID           string `json:"id"`
	NameEnc      []byte `json:"-"`      // encrypted: client name
	EmailEnc     []byte `json:"-"`      // encrypted: email address
	PhoneEnc     []byte `json:"-"`      // encrypted: phone number
	NHIEnc       []byte `json:"-"`      // encrypted: NHI identifier
	Employer     string `json:"employer,omitempty"`
	Notes        string `json:"notes,omitempty"`
	Active       bool   `json:"active"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// PrivateClientResponse is the API response — PHI is decrypted for authorised callers.
type PrivateClientResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	NHI       string `json:"nhi"`
	Employer  string `json:"employer,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Active    bool   `json:"active"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// NewEncryptedClient encrypts PHI fields from the request and returns a PrivateClient
// ready for persistence.
func NewEncryptedClient(req PrivateClientRequest, enc *encryption.Cipher) (*PrivateClient, error) {
	nameEnc, err := enc.Encrypt([]byte(req.Name))
	if err != nil {
		return nil, fmt.Errorf("private: encrypt name: %w", err)
	}
	emailEnc, err := enc.Encrypt([]byte(req.Email))
	if err != nil {
		return nil, fmt.Errorf("private: encrypt email: %w", err)
	}
	phoneEnc, err := enc.Encrypt([]byte(req.Phone))
	if err != nil {
		return nil, fmt.Errorf("private: encrypt phone: %w", err)
	}
	nhiEnc, err := enc.Encrypt([]byte(req.NHI))
	if err != nil {
		return nil, fmt.Errorf("private: encrypt NHI: %w", err)
	}
	return &PrivateClient{
		NameEnc:  nameEnc,
		EmailEnc: emailEnc,
		PhoneEnc: phoneEnc,
		NHIEnc:   nhiEnc,
		Employer: req.Employer,
		Notes:    req.Notes,
		Active:   req.Active,
	}, nil
}

// ToResponse decrypts PHI fields and returns a PrivateClientResponse for the API.
// Decryption errors return an empty string for the affected field rather than
// aborting the response, so partial data is visible to aid debugging.
func (c *PrivateClient) ToResponse(enc *encryption.Cipher) PrivateClientResponse {
	resp := PrivateClientResponse{
		ID:        c.ID,
		Employer:  c.Employer,
		Notes:     c.Notes,
		Active:    c.Active,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if name, err := enc.Decrypt(c.NameEnc); err == nil {
		resp.Name = string(name)
	}
	if email, err := enc.Decrypt(c.EmailEnc); err == nil {
		resp.Email = string(email)
	}
	if phone, err := enc.Decrypt(c.PhoneEnc); err == nil {
		resp.Phone = string(phone)
	}
	if nhi, err := enc.Decrypt(c.NHIEnc); err == nil {
		resp.NHI = string(nhi)
	}
	return resp
}

// Invoice represents a private practice billing invoice.
type Invoice struct {
	ID            string `json:"id"`
	ClientNHI     string `json:"clientNhi"`
	InvoiceNumber string `json:"invoiceNumber"`
	Sessions      int    `json:"sessions"`
	SessionFee    int    `json:"sessionFee"`
	TotalAmount   int    `json:"totalAmount"`
	TaxAmount     int    `json:"taxAmount"`
	Status        string `json:"status"` // draft, sent, paid, overdue
	DueDate       int64  `json:"dueDate"`
	PaidDate      int64  `json:"paidDate,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}
