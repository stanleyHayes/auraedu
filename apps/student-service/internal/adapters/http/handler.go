package http

import (
	"net/http"

	"github.com/auraedu/student-service/internal/application"
)

// Handler adapts HTTP to the student use cases. No business logic here (agent_plan §5).
type Handler struct {
	svc *application.Service
}

func NewHandler(svc *application.Service) *Handler { return &Handler{svc: svc} }

// Register mounts the service routes.
// TODO(AURA): implement per contracts/openapi/student.v1.yaml; enforce
// authenticated → tenant → RBAC → feature-flag → ownership before each action.
func (h *Handler) Register(mux *http.ServeMux) {
	_ = h
	_ = mux
	// mux.HandleFunc("GET /api/v1/students", h.list)
}
