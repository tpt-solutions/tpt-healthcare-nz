package inventory

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines persistence for inventory data.
type Repository interface {
	// Stock items
	CreateStockItem(ctx context.Context, item StockItem) (*StockItem, error)
	GetStockItem(ctx context.Context, id uuid.UUID) (*StockItem, error)
	ListStockItems(ctx context.Context, tenantID uuid.UUID) ([]StockItem, error)
	UpdateStockItem(ctx context.Context, item StockItem) (*StockItem, error)

	// Movements — append-only; no update or delete
	RecordMovement(ctx context.Context, m StockMovement) (*StockMovement, error)
	ListMovements(ctx context.Context, stockItemID uuid.UUID, since time.Time) ([]StockMovement, error)

	// Purchase orders
	CreatePurchaseOrder(ctx context.Context, po PurchaseOrder) (*PurchaseOrder, error)
	GetPurchaseOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error)
	ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID) ([]PurchaseOrder, error)
	UpdatePurchaseOrderStatus(ctx context.Context, id uuid.UUID, status PurchaseOrderStatus) error
	ReceiveLine(ctx context.Context, lineID uuid.UUID, quantityReceived int64) error

	// Cold chain
	RecordTemp(ctx context.Context, log ColdChainLog) (*ColdChainLog, error)
	ListBreaches(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]ColdChainLog, error)

	// Alerts
	LowStockItems(ctx context.Context, tenantID uuid.UUID) ([]StockItem, error)
	ExpiringItems(ctx context.Context, tenantID uuid.UUID, within time.Duration) ([]StockItem, error)
}
