package contacts

import (
	"github.com/gin-gonic/gin"
	"journey-safety/backend/internal/auth"
	"journey-safety/backend/internal/platform/httpapi"
)

type Handler struct{ service *Service }

func NewHandler(s *Service) *Handler { return &Handler{s} }
func (h *Handler) Invite(c *gin.Context) {
	var in struct {
		Email string `json:"email"`
	}
	if !httpapi.Bind(c, &in) {
		return
	}
	i, token, err := h.service.Invite(c.Request.Context(), auth.Current(c), in.Email)
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	httpapi.Data(c, 201, gin.H{"invitation": i, "invitation_token": token})
}
func (h *Handler) Invitations(c *gin.Context) {
	items, err := h.service.Invitations(c.Request.Context(), auth.Current(c))
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	httpapi.Data(c, 200, items)
}
func (h *Handler) Accept(c *gin.Context) {
	id, ok := httpapi.ID(c)
	if !ok {
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if !httpapi.Bind(c, &in) {
		return
	}
	item, err := h.service.Accept(c.Request.Context(), auth.Current(c), id, in.Token)
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	httpapi.Data(c, 200, item)
}
func (h *Handler) Decline(c *gin.Context) { h.finish(c, "DECLINED") }
func (h *Handler) Cancel(c *gin.Context)  { h.finish(c, "CANCELED") }
func (h *Handler) finish(c *gin.Context, status string) {
	id, ok := httpapi.ID(c)
	if !ok {
		return
	}
	if err := h.service.Finish(c.Request.Context(), auth.Current(c), id, status); err != nil {
		httpapi.Fail(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) List(c *gin.Context)      { h.list(c, false) }
func (h *Handler) TrustedBy(c *gin.Context) { h.list(c, true) }
func (h *Handler) list(c *gin.Context, incoming bool) {
	items, err := h.service.List(c.Request.Context(), auth.Current(c), incoming)
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	httpapi.Data(c, 200, items)
}
func (h *Handler) Revoke(c *gin.Context) {
	id, ok := httpapi.ID(c)
	if !ok {
		return
	}
	if err := h.service.Revoke(c.Request.Context(), auth.Current(c), id); err != nil {
		httpapi.Fail(c, err)
		return
	}
	c.Status(204)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := httpapi.ID(c)
	if !ok {
		return
	}
	var in struct {
		Label string `json:"label"`
	}
	if !httpapi.Bind(c, &in) {
		return
	}
	item, err := h.service.UpdateLabel(c.Request.Context(), auth.Current(c), id, in.Label)
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	httpapi.Data(c, 200, item)
}
