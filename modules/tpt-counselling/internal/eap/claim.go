// Package eap provides Employee Assistance Programme billing for counselling.
package eap

import "time"

type EAPClaim struct {
	ID            string    `json:"id"`
	ClientNHI     string    `json:"clientNhi"`
	ProviderHPI   string    `json:"providerHpi"`
	PracticeID    string    `json:"practiceId"`
	EAPProvider   string    `json:"eapProvider"`   // EAP service provider name
	SessionCount  int       `json:"sessionCount"`
	SessionFee    int       `json:"sessionFee"`    // per session in NZ cents
	TotalFee      int       `json:"totalFee"`
	Status        string    `json:"status"`         // submitted, approved, paid, rejected
	Reference     string    `json:"reference"`      // EAP claim reference number
	InvoiceNumber string    `json:"invoiceNumber,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}