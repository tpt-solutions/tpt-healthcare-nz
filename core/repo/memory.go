package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is an in-process, non-persistent implementation of Store.
// It is intended for local development and unit tests where a PostgreSQL
// instance is not available; data does not survive process restarts.
type MemoryStore struct {
	mu    sync.RWMutex
	seq   int64
	items map[string]*memoryItem
}

type memoryItem struct {
	tenantID     uuid.UUID
	resourceType string
	resourceID   string
	versionID    int64
	data         json.RawMessage
	updatedAt    time.Time
	deleted      bool
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: make(map[string]*memoryItem)}
}

func memoryKey(tenantID, resourceType, resourceID string) string {
	return tenantID + "/" + resourceType + "/" + resourceID
}

// Create implements Store.
func (s *MemoryStore) Create(ctx context.Context, tenantID, resourceType, resourceID string, data json.RawMessage) (*ResourceMeta, error) {
	if resourceID == "" {
		resourceID = uuid.NewString()
	}
	tid, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := memoryKey(tenantID, resourceType, resourceID)
	if existing, ok := s.items[key]; ok && !existing.deleted {
		return nil, fmt.Errorf("repo: %s/%s already exists", resourceType, resourceID)
	}

	s.seq++
	now := time.Now().UTC()
	item := &memoryItem{
		tenantID:     tid,
		resourceType: resourceType,
		resourceID:   resourceID,
		versionID:    1,
		data:         append(json.RawMessage(nil), data...),
		updatedAt:    now,
	}
	s.items[key] = item

	return itemMeta(item), nil
}

// Read implements Store.
func (s *MemoryStore) Read(ctx context.Context, tenantID, resourceType, resourceID string) (json.RawMessage, *ResourceMeta, error) {
	if _, err := uuid.Parse(tenantID); err != nil {
		return nil, nil, fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[memoryKey(tenantID, resourceType, resourceID)]
	if !ok || item.deleted {
		return nil, nil, fmt.Errorf("repo: %s/%s not found: %w", resourceType, resourceID, ErrNotFound)
	}
	return append(json.RawMessage(nil), item.data...), itemMeta(item), nil
}

// Update implements Store.
func (s *MemoryStore) Update(ctx context.Context, tenantID, resourceType, resourceID string, data json.RawMessage) (*ResourceMeta, error) {
	if _, err := uuid.Parse(tenantID); err != nil {
		return nil, fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := memoryKey(tenantID, resourceType, resourceID)
	item, ok := s.items[key]
	if !ok || item.deleted {
		return nil, fmt.Errorf("repo: %s/%s not found: %w", resourceType, resourceID, ErrNotFound)
	}

	item.versionID++
	item.data = append(json.RawMessage(nil), data...)
	item.updatedAt = time.Now().UTC()

	return itemMeta(item), nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(ctx context.Context, tenantID, resourceType, resourceID string) error {
	if _, err := uuid.Parse(tenantID); err != nil {
		return fmt.Errorf("repo: invalid tenantID %q: %w", tenantID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[memoryKey(tenantID, resourceType, resourceID)]
	if !ok || item.deleted {
		return fmt.Errorf("repo: %s/%s not found: %w", resourceType, resourceID, ErrNotFound)
	}
	item.deleted = true
	item.updatedAt = time.Now().UTC()
	return nil
}

// Search implements Store. Matching is a best-effort, in-memory approximation
// of FHIR search: each requested parameter is matched against the resource's
// top-level JSON field as a case-insensitive substring match.
func (s *MemoryStore) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matched []*memoryItem
	for _, item := range s.items {
		if item.deleted || item.tenantID != params.TenantID || item.resourceType != params.ResourceType {
			continue
		}
		if matchesParams(item.data, params.Params) {
			matched = append(matched, item)
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].updatedAt.After(matched[j].updatedAt)
	})

	total := len(matched)
	pageSize := params.Count
	if pageSize <= 0 {
		pageSize = 20
	}
	start := params.Offset
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	resources := make([]json.RawMessage, 0, end-start)
	for _, item := range matched[start:end] {
		resources = append(resources, append(json.RawMessage(nil), item.data...))
	}

	nextOffset := -1
	if end < total {
		nextOffset = end
	}

	return &SearchResult{Resources: resources, Total: total, NextOffset: nextOffset}, nil
}

func matchesParams(data json.RawMessage, params map[string][]string) bool {
	if len(params) == 0 {
		return true
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return false
	}
	for key, values := range params {
		field, ok := fields[key]
		if !ok {
			return false
		}
		fieldText := strings.ToLower(fmt.Sprintf("%v", field))
		found := false
		for _, v := range values {
			if strings.Contains(fieldText, strings.ToLower(v)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func itemMeta(item *memoryItem) *ResourceMeta {
	return &ResourceMeta{
		ResourceType: item.resourceType,
		ResourceID:   item.resourceID,
		VersionID:    fmt.Sprintf("%d", item.versionID),
		TenantID:     item.tenantID,
		LastUpdated:  item.updatedAt,
	}
}
