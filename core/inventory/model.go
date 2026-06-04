// Package inventory manages medical supply stock for the tpt-healthcare platform.
// Covers vaccines, medications, consumables, and equipment. Stock movements are
// an immutable append-only ledger. Cold-chain logs record temperature readings
// for items requiring refrigeration (vaccines, biological products).
package inventory

import (
	"time"

	"github.com/google/uuid"
)

// Category classifies a stock item.
type Category string

const (
	CategoryVaccine    Category = "vaccine"
	CategoryMedication Category = "medication"
	CategoryConsumable Category = "consumable"
	CategoryEquipment  Category = "equipment"
)

// StockItem is a master record for a stocked product.
type StockItem struct {
	ID           uuid.UUID `db:"id"            json:"id"`
	TenantID     uuid.UUID `db:"tenant_id"     json:"tenant_id"`
	SKU          string    `db:"sku"           json:"sku"`
	Name         string    `db:"name"          json:"name"`
	Category     Category  `db:"category"      json:"category"`
	Unit         string    `db:"unit"          json:"unit"` // e.g. "vial", "box", "unit"
	// QuantityOnHand is maintained by the movement ledger (not directly writable).
	QuantityOnHand int64 `db:"quantity_on_hand" json:"quantity_on_hand"`
	// ReorderPoint triggers a low-stock alert when QuantityOnHand falls to this level.
	ReorderPoint int64 `db:"reorder_point" json:"reorder_point"`
	// StorageTempMinC and StorageTempMaxC define the cold-chain range (nil = no cold chain).
	StorageTempMinC *float64 `db:"storage_temp_min_c" json:"storage_temp_min_c,omitempty"`
	StorageTempMaxC *float64 `db:"storage_temp_max_c" json:"storage_temp_max_c,omitempty"`
	// FHIRSupplyDeliveryRef links to the FHIR SupplyDelivery resource for NIR vaccine traceability.
	FHIRSupplyDeliveryRef string    `db:"fhir_supply_delivery_ref" json:"fhir_supply_delivery_ref,omitempty"`
	ExpiryDate            *time.Time `db:"expiry_date"    json:"expiry_date,omitempty"`
	CreatedAt             time.Time  `db:"created_at"     json:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"     json:"updated_at"`
}

// MovementType classifies a stock movement.
type MovementType string

const (
	MovementReceive       MovementType = "receive"
	MovementConsume       MovementType = "consume"
	MovementAdjust        MovementType = "adjust"
	MovementTransfer      MovementType = "transfer"
	MovementDiscardExpired MovementType = "discard_expired"
)

// StockMovement is an immutable ledger entry recording a quantity change.
// Movements are never updated or deleted; corrections are made with adjustment entries.
type StockMovement struct {
	ID           uuid.UUID    `db:"id"            json:"id"`
	TenantID     uuid.UUID    `db:"tenant_id"     json:"tenant_id"`
	StockItemID  uuid.UUID    `db:"stock_item_id" json:"stock_item_id"`
	Type         MovementType `db:"type"          json:"type"`
	// QuantityDelta is positive for receives/adjustments-in and negative for consumes/discards.
	QuantityDelta int64     `db:"quantity_delta"  json:"quantity_delta"`
	PerformedBy   string    `db:"performed_by"    json:"performed_by"` // principal ID
	Notes         string    `db:"notes"           json:"notes,omitempty"`
	// EncounterRef links a consumption movement to the clinical encounter that used the item.
	EncounterRef  string    `db:"encounter_ref"   json:"encounter_ref,omitempty"`
	CreatedAt     time.Time `db:"created_at"      json:"created_at"`
}

// PurchaseOrderStatus tracks the lifecycle of a supplier order.
type PurchaseOrderStatus string

const (
	POStatusDraft              PurchaseOrderStatus = "draft"
	POStatusSent               PurchaseOrderStatus = "sent"
	POStatusPartiallyReceived  PurchaseOrderStatus = "partially_received"
	POStatusReceived           PurchaseOrderStatus = "received"
	POStatusCancelled          PurchaseOrderStatus = "cancelled"
)

// PurchaseOrder represents a supplier order for stock replenishment.
type PurchaseOrder struct {
	ID           uuid.UUID           `db:"id"           json:"id"`
	TenantID     uuid.UUID           `db:"tenant_id"    json:"tenant_id"`
	SupplierName string              `db:"supplier_name" json:"supplier_name"`
	Status       PurchaseOrderStatus `db:"status"       json:"status"`
	OrderedAt    *time.Time          `db:"ordered_at"   json:"ordered_at,omitempty"`
	ExpectedAt   *time.Time          `db:"expected_at"  json:"expected_at,omitempty"`
	ReceivedAt   *time.Time          `db:"received_at"  json:"received_at,omitempty"`
	Notes        string              `db:"notes"        json:"notes,omitempty"`
	CreatedAt    time.Time           `db:"created_at"   json:"created_at"`
	Lines        []PurchaseOrderLine `db:"-"            json:"lines,omitempty"`
}

// PurchaseOrderLine is a single item on a purchase order.
type PurchaseOrderLine struct {
	ID              uuid.UUID `db:"id"              json:"id"`
	PurchaseOrderID uuid.UUID `db:"purchase_order_id" json:"purchase_order_id"`
	StockItemID     uuid.UUID `db:"stock_item_id"   json:"stock_item_id"`
	QuantityOrdered int64     `db:"quantity_ordered" json:"quantity_ordered"`
	QuantityReceived int64    `db:"quantity_received" json:"quantity_received"`
	UnitCostCents   int64     `db:"unit_cost_cents"  json:"unit_cost_cents"`
}

// ColdChainLog records a temperature sensor reading for a stock item.
type ColdChainLog struct {
	ID          uuid.UUID `db:"id"           json:"id"`
	TenantID    uuid.UUID `db:"tenant_id"    json:"tenant_id"`
	StockItemID uuid.UUID `db:"stock_item_id" json:"stock_item_id"`
	TempC       float64   `db:"temp_c"       json:"temp_c"`
	RecordedAt  time.Time `db:"recorded_at"  json:"recorded_at"`
	// Breach is true when TempC was outside the item's storage temperature range.
	Breach      bool      `db:"breach"       json:"breach"`
	AlarmSent   bool      `db:"alarm_sent"   json:"alarm_sent"`
}
