package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/charlesduarte/net-control/backend/internal/platform/httpserver"
)

type Handler struct {
	service *Service
	tokens  *TokenManager
}

func NewHandler(service *Service, tokens *TokenManager) *Handler {
	return &Handler{service: service, tokens: tokens}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.Handle("GET /api/v1/auth/me", Authenticate(h.tokens, http.HandlerFunc(h.me)))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Tenant   string `json:"tenant"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if strings.TrimSpace(input.Tenant) == "" || strings.TrimSpace(input.Email) == "" || input.Password == "" {
		httpserver.WriteError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Tenant, email and password are required")
		return
	}

	token, principal, err := h.service.Login(r.Context(), input.Tenant, input.Email, input.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		httpserver.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid credentials")
		return
	}
	if err != nil {
		httpserver.WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": token, "token_type": "Bearer", "expires_at": principal.ExpiresAt,
	})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	httpserver.WriteJSON(w, http.StatusOK, map[string]any{"user": principal})
}
