package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/example/tenant-image-pipeline/internal/infrai"
	"github.com/example/tenant-image-pipeline/internal/onboarding"
)

type gateway struct {
	tenants    *onboarding.Registry
	compressor *infrai.Compressor
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	app := &gateway{
		tenants:    onboarding.NewRegistry(),
		compressor: &infrai.Compressor{APIKey: apiKey},
	}
	addr := envOr("ADDR", ":8080")
	log.Printf("tenant image gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, app.routes()))
}

func (g *gateway) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /tenants", g.onboard)
	mux.HandleFunc("POST /admin/tenants/{tenantID}/{action}", g.changeStatus)
	mux.HandleFunc("POST /tenants/{tenantID}/images", g.compressImage)
	return mux
}

func (g *gateway) onboard(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&input); err != nil || input.TenantID == "" {
		writeError(w, http.StatusBadRequest, "tenant_id is required")
		return
	}
	tenant, err := g.tenants.Onboard(input.TenantID)
	if errors.Is(err, onboarding.ErrTenantExists) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tenant)
}

func (g *gateway) changeStatus(w http.ResponseWriter, r *http.Request) {
	actions := map[string]onboarding.Status{
		"activate": onboarding.Active,
		"suspend":  onboarding.Suspended,
		"close":    onboarding.Closed,
	}
	next, ok := actions[r.PathValue("action")]
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown lifecycle action")
		return
	}
	tenant, err := g.tenants.Transition(r.PathValue("tenantID"), next)
	if err != nil {
		status := http.StatusConflict
		if errors.Is(err, onboarding.ErrTenantNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (g *gateway) compressImage(w http.ResponseWriter, r *http.Request) {
	tenantID := r.PathValue("tenantID")
	if err := g.tenants.AuthorizeImage(tenantID); err != nil {
		status := http.StatusConflict
		if errors.Is(err, onboarding.ErrTenantNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	requestID := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "Idempotency-Key is required")
		return
	}
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "multipart image is required")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}
	defer file.Close()
	image, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read image")
		return
	}
	data, err := g.compressor.Compress(r.Context(), header.Filename, image, tenantID+":"+requestID)
	if err != nil {
		var apiErr *infrai.APIError
		if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
			writeError(w, apiErr.HTTPStatus, apiErr.Message)
			return
		}
		writeError(w, http.StatusBadGateway, "image processing request failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "image": data})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
