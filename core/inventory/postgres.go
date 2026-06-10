package inventory

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository against a PostgreSQL pool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateStockItem(ctx context.Context, item StockItem) (*StockItem, error) {
	const q = `
		INSERT INTO stock_items (id, tenant_id, sku, name, category, unit, reorder_point,
		                         storage_temp_min_c, storage_temp_max_c,
		                         fhir_supply_delivery_ref, expiry_date)
		VALUES (gen_random_uuid(), @tenant_id, @sku, @name, @category, @unit, @reorder_point,
		        @min_c, @max_c, @fhir_ref, @expiry)
		RETURNING id, tenant_id, sku, name, category, unit,
		          quantity_on_hand, reorder_point,
		          storage_temp_min_c, storage_temp_max_c,
		          fhir_supply_delivery_ref, expiry_date, created_at, updated_at`
	row := r.pool.QueryRow(ctx, q, map[string]any{
		"tenant_id":    item.TenantID,
		"sku":          item.SKU,
		"name":         item.Name,
		"category":     item.Category,
		"unit":         item.Unit,
		"reorder_point": item.ReorderPoint,
		"min_c":        item.StorageTempMinC,
		"max_c":        item.StorageTempMaxC,
		"fhir_ref":     item.FHIRSupplyDeliveryRef,
		"expiry":       item.ExpiryDate,
	})
	return scanStockItem(row)
}

func (r *PostgresRepository) GetStockItem(ctx context.Context, id uuid.UUID) (*StockItem, error) {
	const q = `
		SELECT id, tenant_id, sku, name, category, unit,
		       quantity_on_hand, reorder_point,
		       storage_temp_min_c, storage_temp_max_c,
		       fhir_supply_delivery_ref, expiry_date, created_at, updated_at
		FROM stock_items WHERE id = @id`
	return scanStockItem(r.pool.QueryRow(ctx, q, map[string]any{"id": id}))
}

func (r *PostgresRepository) ListStockItems(ctx context.Context, tenantID uuid.UUID) ([]StockItem, error) {
	const q = `
		SELECT id, tenant_id, sku, name, category, unit,
		       quantity_on_hand, reorder_point,
		       storage_temp_min_c, storage_temp_max_c,
		       fhir_supply_delivery_ref, expiry_date, created_at, updated_at
		FROM stock_items WHERE tenant_id = @tenant_id ORDER BY name`
	rows, err := r.pool.Query(ctx, q, map[string]any{"tenant_id": tenantID})
	if err != nil {
		return nil, fmt.Errorf("inventory list stock items: %w", err)
	}
	defer rows.Close()
	var result []StockItem
	for rows.Next() {
		item, err := scanStockItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) UpdateStockItem(ctx context.Context, item StockItem) (*StockItem, error) {
	const q = `
		UPDATE stock_items
		SET name = @name, category = @category, unit = @unit,
		    reorder_point = @reorder_point,
		    storage_temp_min_c = @min_c, storage_temp_max_c = @max_c,
		    fhir_supply_delivery_ref = @fhir_ref, expiry_date = @expiry,
		    updated_at = NOW()
		WHERE id = @id
		RETURNING id, tenant_id, sku, name, category, unit,
		          quantity_on_hand, reorder_point,
		          storage_temp_min_c, storage_temp_max_c,
		          fhir_supply_delivery_ref, expiry_date, created_at, updated_at`
	return scanStockItem(r.pool.QueryRow(ctx, q, map[string]any{
		"id":            item.ID,
		"name":          item.Name,
		"category":      item.Category,
		"unit":          item.Unit,
		"reorder_point": item.ReorderPoint,
		"min_c":         item.StorageTempMinC,
		"max_c":         item.StorageTempMaxC,
		"fhir_ref":      item.FHIRSupplyDeliveryRef,
		"expiry":        item.ExpiryDate,
	}))
}

func (r *PostgresRepository) RecordMovement(ctx context.Context, m StockMovement) (*StockMovement, error) {
	const q = `
		INSERT INTO stock_movements (id, tenant_id, stock_item_id, type,
		                             quantity_delta, performed_by, notes, encounter_ref)
		VALUES (gen_random_uuid(), @tenant_id, @stock_item_id, @type,
		        @quantity_delta, @performed_by, @notes, @encounter_ref)
		RETURNING id, tenant_id, stock_item_id, type,
		          quantity_delta, performed_by, notes, encounter_ref, created_at`
	row := r.pool.QueryRow(ctx, q, map[string]any{
		"tenant_id":     m.TenantID,
		"stock_item_id": m.StockItemID,
		"type":          m.Type,
		"quantity_delta": m.QuantityDelta,
		"performed_by":  m.PerformedBy,
		"notes":         m.Notes,
		"encounter_ref": m.EncounterRef,
	})
	return scanMovement(row)
}

func (r *PostgresRepository) ListMovements(ctx context.Context, stockItemID uuid.UUID, since time.Time) ([]StockMovement, error) {
	const q = `
		SELECT id, tenant_id, stock_item_id, type,
		       quantity_delta, performed_by, notes, encounter_ref, created_at
		FROM stock_movements
		WHERE stock_item_id = @id AND created_at >= @since
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, map[string]any{"id": stockItemID, "since": since})
	if err != nil {
		return nil, fmt.Errorf("inventory list movements: %w", err)
	}
	defer rows.Close()
	var result []StockMovement
	for rows.Next() {
		m, err := scanMovement(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *m)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) CreatePurchaseOrder(ctx context.Context, po PurchaseOrder) (*PurchaseOrder, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("inventory create po: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var out PurchaseOrder
	err = tx.QueryRow(ctx, `
		INSERT INTO purchase_orders (id, tenant_id, supplier_name, status, ordered_at, expected_at, notes)
		VALUES (gen_random_uuid(), @tenant_id, @supplier_name, @status, @ordered_at, @expected_at, @notes)
		RETURNING id, tenant_id, supplier_name, status, ordered_at, expected_at, received_at, notes, created_at`,
		map[string]any{
			"tenant_id":    po.TenantID,
			"supplier_name": po.SupplierName,
			"status":       po.Status,
			"ordered_at":   po.OrderedAt,
			"expected_at":  po.ExpectedAt,
			"notes":        po.Notes,
		},
	).Scan(&out.ID, &out.TenantID, &out.SupplierName, &out.Status,
		&out.OrderedAt, &out.ExpectedAt, &out.ReceivedAt, &out.Notes, &out.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inventory create po header: %w", err)
	}

	for i, line := range po.Lines {
		var l PurchaseOrderLine
		err = tx.QueryRow(ctx, `
			INSERT INTO purchase_order_lines (id, purchase_order_id, stock_item_id,
			                                  quantity_ordered, unit_cost_cents)
			VALUES (gen_random_uuid(), @po_id, @stock_item_id, @qty, @cost)
			RETURNING id, purchase_order_id, stock_item_id,
			          quantity_ordered, quantity_received, unit_cost_cents`,
			map[string]any{
				"po_id":        out.ID,
				"stock_item_id": line.StockItemID,
				"qty":          line.QuantityOrdered,
				"cost":         line.UnitCostCents,
			},
		).Scan(&l.ID, &l.PurchaseOrderID, &l.StockItemID,
			&l.QuantityOrdered, &l.QuantityReceived, &l.UnitCostCents)
		if err != nil {
			return nil, fmt.Errorf("inventory create po line %d: %w", i, err)
		}
		out.Lines = append(out.Lines, l)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("inventory create po commit: %w", err)
	}
	return &out, nil
}

func (r *PostgresRepository) GetPurchaseOrder(ctx context.Context, id uuid.UUID) (*PurchaseOrder, error) {
	var po PurchaseOrder
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, supplier_name, status,
		       ordered_at, expected_at, received_at, notes, created_at
		FROM purchase_orders WHERE id = @id`,
		map[string]any{"id": id},
	).Scan(&po.ID, &po.TenantID, &po.SupplierName, &po.Status,
		&po.OrderedAt, &po.ExpectedAt, &po.ReceivedAt, &po.Notes, &po.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("inventory get po: %w", err)
	}
	lines, err := r.listPOLines(ctx, id)
	if err != nil {
		return nil, err
	}
	po.Lines = lines
	return &po, nil
}

func (r *PostgresRepository) ListPurchaseOrders(ctx context.Context, tenantID uuid.UUID) ([]PurchaseOrder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, supplier_name, status,
		       ordered_at, expected_at, received_at, notes, created_at
		FROM purchase_orders WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
		map[string]any{"tenant_id": tenantID})
	if err != nil {
		return nil, fmt.Errorf("inventory list pos: %w", err)
	}
	defer rows.Close()
	var result []PurchaseOrder
	for rows.Next() {
		var po PurchaseOrder
		if err := rows.Scan(&po.ID, &po.TenantID, &po.SupplierName, &po.Status,
			&po.OrderedAt, &po.ExpectedAt, &po.ReceivedAt, &po.Notes, &po.CreatedAt); err != nil {
			return nil, fmt.Errorf("inventory scan po: %w", err)
		}
		result = append(result, po)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) UpdatePurchaseOrderStatus(ctx context.Context, id uuid.UUID, status PurchaseOrderStatus) error {
	extra := ""
	if status == POStatusReceived {
		extra = ", received_at = NOW()"
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE purchase_orders SET status = @status`+extra+` WHERE id = @id`,
		map[string]any{"id": id, "status": status})
	return err
}

func (r *PostgresRepository) ReceiveLine(ctx context.Context, lineID uuid.UUID, quantityReceived int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE purchase_order_lines
		SET quantity_received = quantity_received + @qty
		WHERE id = @id`,
		map[string]any{"id": lineID, "qty": quantityReceived})
	return err
}

func (r *PostgresRepository) RecordTemp(ctx context.Context, log ColdChainLog) (*ColdChainLog, error) {
	var out ColdChainLog
	err := r.pool.QueryRow(ctx, `
		INSERT INTO cold_chain_logs (id, tenant_id, stock_item_id, temp_c, recorded_at, breach)
		VALUES (gen_random_uuid(), @tenant_id, @stock_item_id, @temp_c, @recorded_at, @breach)
		RETURNING id, tenant_id, stock_item_id, temp_c, recorded_at, breach, alarm_sent`,
		map[string]any{
			"tenant_id":    log.TenantID,
			"stock_item_id": log.StockItemID,
			"temp_c":       log.TempC,
			"recorded_at":  log.RecordedAt,
			"breach":       log.Breach,
		},
	).Scan(&out.ID, &out.TenantID, &out.StockItemID,
		&out.TempC, &out.RecordedAt, &out.Breach, &out.AlarmSent)
	if err != nil {
		return nil, fmt.Errorf("inventory record temp: %w", err)
	}
	return &out, nil
}

func (r *PostgresRepository) ListBreaches(ctx context.Context, tenantID uuid.UUID, since time.Time) ([]ColdChainLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, stock_item_id, temp_c, recorded_at, breach, alarm_sent
		FROM cold_chain_logs
		WHERE tenant_id = @tenant_id AND breach = true AND recorded_at >= @since
		ORDER BY recorded_at DESC`,
		map[string]any{"tenant_id": tenantID, "since": since})
	if err != nil {
		return nil, fmt.Errorf("inventory list breaches: %w", err)
	}
	defer rows.Close()
	var result []ColdChainLog
	for rows.Next() {
		var l ColdChainLog
		if err := rows.Scan(&l.ID, &l.TenantID, &l.StockItemID,
			&l.TempC, &l.RecordedAt, &l.Breach, &l.AlarmSent); err != nil {
			return nil, fmt.Errorf("inventory scan breach: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) LowStockItems(ctx context.Context, tenantID uuid.UUID) ([]StockItem, error) {
	const q = `
		SELECT id, tenant_id, sku, name, category, unit,
		       quantity_on_hand, reorder_point,
		       storage_temp_min_c, storage_temp_max_c,
		       fhir_supply_delivery_ref, expiry_date, created_at, updated_at
		FROM stock_items
		WHERE tenant_id = @tenant_id AND quantity_on_hand <= reorder_point
		ORDER BY name`
	rows, err := r.pool.Query(ctx, q, map[string]any{"tenant_id": tenantID})
	if err != nil {
		return nil, fmt.Errorf("inventory low stock items: %w", err)
	}
	defer rows.Close()
	var result []StockItem
	for rows.Next() {
		item, err := scanStockItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

func (r *PostgresRepository) ExpiringItems(ctx context.Context, tenantID uuid.UUID, within time.Duration) ([]StockItem, error) {
	cutoff := time.Now().UTC().Add(within)
	const q = `
		SELECT id, tenant_id, sku, name, category, unit,
		       quantity_on_hand, reorder_point,
		       storage_temp_min_c, storage_temp_max_c,
		       fhir_supply_delivery_ref, expiry_date, created_at, updated_at
		FROM stock_items
		WHERE tenant_id = @tenant_id
		  AND expiry_date IS NOT NULL
		  AND expiry_date <= @cutoff
		ORDER BY expiry_date`
	rows, err := r.pool.Query(ctx, q, map[string]any{"tenant_id": tenantID, "cutoff": cutoff})
	if err != nil {
		return nil, fmt.Errorf("inventory expiring items: %w", err)
	}
	defer rows.Close()
	var result []StockItem
	for rows.Next() {
		item, err := scanStockItem(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *item)
	}
	return result, rows.Err()
}

// --- helpers ---

func (r *PostgresRepository) listPOLines(ctx context.Context, poID uuid.UUID) ([]PurchaseOrderLine, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, purchase_order_id, stock_item_id,
		       quantity_ordered, quantity_received, unit_cost_cents
		FROM purchase_order_lines WHERE purchase_order_id = @id`,
		map[string]any{"id": poID})
	if err != nil {
		return nil, fmt.Errorf("inventory list po lines: %w", err)
	}
	defer rows.Close()
	var result []PurchaseOrderLine
	for rows.Next() {
		var l PurchaseOrderLine
		if err := rows.Scan(&l.ID, &l.PurchaseOrderID, &l.StockItemID,
			&l.QuantityOrdered, &l.QuantityReceived, &l.UnitCostCents); err != nil {
			return nil, fmt.Errorf("inventory scan po line: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanStockItem(s scanner) (*StockItem, error) {
	var item StockItem
	if err := s.Scan(
		&item.ID, &item.TenantID, &item.SKU, &item.Name, &item.Category, &item.Unit,
		&item.QuantityOnHand, &item.ReorderPoint,
		&item.StorageTempMinC, &item.StorageTempMaxC,
		&item.FHIRSupplyDeliveryRef, &item.ExpiryDate,
		&item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("inventory scan stock item: %w", err)
	}
	return &item, nil
}

func scanMovement(s scanner) (*StockMovement, error) {
	var m StockMovement
	if err := s.Scan(
		&m.ID, &m.TenantID, &m.StockItemID, &m.Type,
		&m.QuantityDelta, &m.PerformedBy, &m.Notes, &m.EncounterRef,
		&m.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("inventory scan movement: %w", err)
	}
	return &m, nil
}
