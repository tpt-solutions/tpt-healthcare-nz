package inventory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/PhillipC05/tpt-healthcare/core/sms"
)

// Service provides business logic for inventory management.
type Service struct {
	repo           Repository
	bus            *events.Bus
	smsProvider    sms.Provider
	smsAlertPhone  string // E.164 phone for cold-chain breach alerts (practice admin)
	logger         *slog.Logger
}

// NewService constructs a Service.
func NewService(repo Repository, bus *events.Bus, logger *slog.Logger) *Service {
	return &Service{repo: repo, bus: bus, logger: logger}
}

// WithSMS attaches an SMS provider for cold-chain breach alerts.
// alertPhone is the E.164 number of the responsible pharmacy/practice manager.
func (s *Service) WithSMS(provider sms.Provider, alertPhone string) *Service {
	s.smsProvider = provider
	s.smsAlertPhone = alertPhone
	return s
}

// Receive records a stock receipt and fires a domain event.
func (s *Service) Receive(ctx context.Context, stockItemID, tenantID uuid.UUID, qty int64, performedBy, notes string) (*StockMovement, error) {
	m := StockMovement{
		TenantID:      tenantID,
		StockItemID:   stockItemID,
		Type:          MovementReceive,
		QuantityDelta: qty,
		PerformedBy:   performedBy,
		Notes:         notes,
	}
	return s.repo.RecordMovement(ctx, m)
}

// Consume records a stock consumption linked to a clinical encounter.
func (s *Service) Consume(ctx context.Context, stockItemID, tenantID uuid.UUID, qty int64, performedBy, encounterRef string) (*StockMovement, error) {
	m := StockMovement{
		TenantID:      tenantID,
		StockItemID:   stockItemID,
		Type:          MovementConsume,
		QuantityDelta: -qty,
		PerformedBy:   performedBy,
		EncounterRef:  encounterRef,
	}
	mov, err := s.repo.RecordMovement(ctx, m)
	if err != nil {
		return nil, err
	}
	// Check if we've fallen below the reorder point.
	item, err := s.repo.GetStockItem(ctx, stockItemID)
	if err != nil {
		return mov, nil // non-fatal; alert check is best-effort
	}
	if item.QuantityOnHand <= item.ReorderPoint {
		_ = s.bus.Publish(ctx, events.Event{
			Type:    events.EventInventoryLowStock,
			Payload: map[string]any{"stock_item_id": item.ID, "tenant_id": tenantID, "qty": item.QuantityOnHand},
		})
	}
	return mov, nil
}

// RecordTemp records a cold-chain temperature reading and fires a breach event
// if the temperature is outside the item's allowed range.
func (s *Service) RecordTemp(ctx context.Context, stockItemID, tenantID uuid.UUID, tempC float64) (*ColdChainLog, error) {
	item, err := s.repo.GetStockItem(ctx, stockItemID)
	if err != nil {
		return nil, fmt.Errorf("inventory record temp: %w", err)
	}

	breach := false
	if item.StorageTempMinC != nil && tempC < *item.StorageTempMinC {
		breach = true
	}
	if item.StorageTempMaxC != nil && tempC > *item.StorageTempMaxC {
		breach = true
	}

	log := ColdChainLog{
		TenantID:    tenantID,
		StockItemID: stockItemID,
		TempC:       tempC,
		RecordedAt:  time.Now().UTC(),
		Breach:      breach,
	}
	rec, err := s.repo.RecordTemp(ctx, log)
	if err != nil {
		return nil, err
	}

	if breach {
		s.bus.Publish(ctx, events.Event{
			Type: events.EventInventoryColdChainBreach,
			Payload: map[string]any{
				"stock_item_id": item.ID,
				"tenant_id":     tenantID,
				"temp_c":        tempC,
				"item_name":     item.Name,
			},
		})
		// Send an immediate SMS to the configured alert phone if available.
		if s.smsProvider != nil && s.smsAlertPhone != "" {
			msg := fmt.Sprintf("COLD CHAIN BREACH: %s recorded %.1f°C — check storage immediately.", item.Name, tempC)
			if _, smsErr := s.smsProvider.Send(ctx, sms.Message{
				To:        s.smsAlertPhone,
				Body:      msg,
				Reference: "cold-chain-" + item.ID.String(),
			}); smsErr != nil {
				s.logger.WarnContext(ctx, "cold-chain breach SMS alert failed",
					slog.String("item", item.Name),
					slog.String("error", smsErr.Error()),
				)
			}
		}
	}
	return rec, nil
}

// --- River jobs ---

// AlertArgs is the River job payload for periodic low-stock + expiry checks.
type AlertArgs struct {
	TenantID uuid.UUID `json:"tenant_id"`
}

func (AlertArgs) Kind() string { return "inventory.check_alerts" }

// AlertWorker runs the low-stock and expiry checks for a tenant.
type AlertWorker struct {
	river.WorkerDefaults[AlertArgs]
	service *Service
	logger  *slog.Logger
}

// NewAlertWorker constructs an AlertWorker.
func NewAlertWorker(service *Service, logger *slog.Logger) *AlertWorker {
	return &AlertWorker{service: service, logger: logger}
}

// Work checks for low-stock and expiring items and fires domain events.
func (w *AlertWorker) Work(ctx context.Context, job *river.Job[AlertArgs]) error {
	lowStock, err := w.service.repo.LowStockItems(ctx, job.Args.TenantID)
	if err != nil {
		return fmt.Errorf("inventory alert worker low stock: %w", err)
	}
	for _, item := range lowStock {
		w.service.bus.Publish(ctx, events.Event{
			Type: events.EventInventoryLowStock,
			Payload: map[string]any{
				"stock_item_id": item.ID,
				"tenant_id":     job.Args.TenantID,
				"qty":           item.QuantityOnHand,
				"item_name":     item.Name,
			},
		})
	}

	expiring, err := w.service.repo.ExpiringItems(ctx, job.Args.TenantID, 30*24*time.Hour)
	if err != nil {
		return fmt.Errorf("inventory alert worker expiry: %w", err)
	}
	for _, item := range expiring {
		w.logger.WarnContext(ctx, "inventory: item expiring soon",
			"id", item.ID, "name", item.Name, "expiry", item.ExpiryDate)
	}

	w.logger.InfoContext(ctx, "inventory: alert check complete",
		"low_stock", len(lowStock), "expiring_soon", len(expiring))
	return nil
}
