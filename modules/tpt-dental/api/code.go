package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-dental/internal/fdi"
)

// CodeHandler handles FDI tooth reference data lookups (non-clinical).
type CodeHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// LookupTooth returns metadata for a single FDI tooth code.
func (h *CodeHandler) LookupTooth(w http.ResponseWriter, r *http.Request) {
	fdiCode := r.PathValue("fdiCode")
	if fdiCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "MISSING_CODE", Message: "FDI tooth code is required",
		})
		return
	}

	tooth, err := fdi.LookupTooth(fdiCode)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code: "INVALID_TOOTH", Message: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, tooth)
}

// Surfaces returns all standard tooth surface codes and their descriptions.
func (h *CodeHandler) Surfaces(w http.ResponseWriter, r *http.Request) {
	surfaces := []map[string]string{
		{"code": "M", "name": fdi.SurfaceName(fdi.SurfaceMesial), "description": "Towards midline (mesial)"},
		{"code": "D", "name": fdi.SurfaceName(fdi.SurfaceDistal), "description": "Away from midline (distal)"},
		{"code": "O", "name": fdi.SurfaceName(fdi.SurfaceOcclusal), "description": "Biting surface (posterior teeth)"},
		{"code": "I", "name": fdi.SurfaceName(fdi.SurfaceIncisal), "description": "Biting edge (anterior teeth)"},
		{"code": "B", "name": fdi.SurfaceName(fdi.SurfaceBuccal), "description": "Cheek side (posterior teeth)"},
		{"code": "L", "name": fdi.SurfaceName(fdi.SurfaceLingual), "description": "Tongue side"},
		{"code": "P", "name": fdi.SurfaceName(fdi.SurfacePalatal), "description": "Palate side (upper teeth)"},
		{"code": "La", "name": fdi.SurfaceName(fdi.SurfaceLabial), "description": "Lip side (anterior teeth)"},
		{"code": "C", "name": fdi.SurfaceName(fdi.SurfaceCervical), "description": "Cervical (gum margin)"},
	}
	writeJSON(w, http.StatusOK, surfaces)
}

// AllTeeth returns all 32 permanent FDI tooth codes with metadata.
func (h *CodeHandler) AllTeeth(w http.ResponseWriter, r *http.Request) {
	codes := fdi.AllPermanentTeeth()
	teeth := make([]fdi.Tooth, 0, len(codes))
	for _, code := range codes {
		t, err := fdi.LookupTooth(code)
		if err != nil {
			h.logger.Error("unexpected tooth lookup error", slog.String("code", code), slog.Any("error", err))
			continue
		}
		teeth = append(teeth, t)
	}
	writeJSON(w, http.StatusOK, teeth)
}

// AllDeciduous returns all 20 deciduous FDI tooth codes with metadata.
func (h *CodeHandler) AllDeciduous(w http.ResponseWriter, r *http.Request) {
	codes := fdi.AllDeciduousTeeth()
	teeth := make([]fdi.Tooth, 0, len(codes))
	for _, code := range codes {
		t, err := fdi.LookupTooth(code)
		if err != nil {
			h.logger.Error("unexpected tooth lookup error", slog.String("code", code), slog.Any("error", err))
			continue
		}
		teeth = append(teeth, t)
	}
	writeJSON(w, http.StatusOK, teeth)
}

// Ensure http.Handler interface is satisfied.
var _ http.Handler = (*CodeHandler)(nil)

func (h *CodeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Default handler routing — handled by server.go route registration
	http.NotFound(w, r)
}

// Ensure ChartHandler, ProcedureHandler, ACCHandler also implement http.Handler.
var _ http.Handler = (*ChartHandler)(nil)
var _ http.Handler = (*ProcedureHandler)(nil)
var _ http.Handler = (*ACCHandler)(nil)

func (h *ChartHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (h *ProcedureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (h *ACCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// Prefix check for the API group name.
const apiGroup = "tpt-dental"

// APIInfo describes the tpt-dental API surface for discovery.
type APIInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// GetAPIInfo returns metadata about the tpt-dental API.
func GetAPIInfo() APIInfo {
	return APIInfo{
		Name:        "tpt-dental",
		Version:     "1.0.0",
		Description: "FDI dental charting, NZ procedure codes, and ACC dental claiming API",
	}
}

// Custom error for invalid FDI code with additional context.
type fdiError struct {
	Code string `json:"code"`
	Msg  string `json:"message"`
	FDI  string `json:"fdiCode,omitempty"`
}

func (e *fdiError) Error() string {
	if e.FDI != "" {
		return fmt.Sprintf("fdi: %s (code: %s)", e.Msg, e.FDI)
	}
	return fmt.Sprintf("fdi: %s", e.Msg)
}
