package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"journey-safety/backend/internal/config"
	"journey-safety/backend/internal/platform/httpapi"
	"journey-safety/backend/internal/platform/security"
)

type Handler struct {
	service *Service
	cfg     config.Config
}

func NewHandler(s *Service, cfg config.Config) *Handler { return &Handler{s, cfg} }
func Current(c *gin.Context) Principal                  { return c.MustGet("principal").(Principal) }
func (h *Handler) RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(h.cfg.CookieName())
		if err != nil {
			httpapi.Fail(c, httpapi.Unauthorized())
			return
		}
		p, err := h.service.Authenticate(c.Request.Context(), token)
		if err != nil {
			httpapi.Fail(c, err)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.Request.Method != http.MethodOptions && !security.Equal(c.GetHeader("X-CSRF-Token"), security.CSRF(token)) {
			httpapi.Fail(c, httpapi.NewError(403, "CSRF_INVALID", "CSRF token không hợp lệ."))
			return
		}
		c.Set("principal", p)
		c.Next()
	}
}
func (h *Handler) cookie(c *gin.Context, token string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if token == "" {
		maxAge = -1
		expires = time.Unix(1, 0)
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: h.cfg.CookieName(), Value: token, Path: "/", HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: maxAge})
}
func (h *Handler) loginResponse(c *gin.Context, status int, r LoginResult) {
	h.cookie(c, r.Token, r.Session.ExpiresAt)
	httpapi.Data(c, status, gin.H{"user": r.User, "csrf_token": r.CSRFToken, "expires_at": r.Session.ExpiresAt})
}
func (h *Handler) Register(c *gin.Context) {
	var in Credentials
	if !httpapi.Bind(c, &in) {
		return
	}
	r, err := h.service.Register(c.Request.Context(), in)
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	h.loginResponse(c, 201, r)
}
func (h *Handler) Login(c *gin.Context) {
	var in struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceName string `json:"device_name"`
	}
	if !httpapi.Bind(c, &in) {
		return
	}
	r, err := h.service.Login(c.Request.Context(), Credentials{Email: in.Email, Password: in.Password, DeviceName: in.DeviceName})
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	h.loginResponse(c, 200, r)
}
func (h *Handler) Session(c *gin.Context) {
	p := Current(c)
	httpapi.Data(c, 200, gin.H{"user": p.User, "csrf_token": security.CSRF(p.Token), "expires_at": p.Session.ExpiresAt})
}
func (h *Handler) Logout(c *gin.Context) {
	p := Current(c)
	if err := h.service.Revoke(c.Request.Context(), p, p.Session.ID); err != nil {
		httpapi.Fail(c, err)
		return
	}
	h.cookie(c, "", time.Time{})
	c.Status(204)
}
func (h *Handler) Sessions(c *gin.Context) {
	items, err := h.service.Sessions(c.Request.Context(), Current(c))
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
	p := Current(c)
	if err := h.service.Revoke(c.Request.Context(), p, id); err != nil {
		httpapi.Fail(c, err)
		return
	}
	if id == p.Session.ID {
		h.cookie(c, "", time.Time{})
	}
	c.Status(204)
}
func (h *Handler) RevokeAll(c *gin.Context) {
	if err := h.service.RevokeAll(c.Request.Context(), Current(c)); err != nil {
		httpapi.Fail(c, err)
		return
	}
	h.cookie(c, "", time.Time{})
	c.Status(204)
}
func (h *Handler) Me(c *gin.Context) { httpapi.Data(c, 200, Current(c).User) }
func (h *Handler) UpdateMe(c *gin.Context) {
	var in struct {
		DisplayName string `json:"display_name"`
	}
	if !httpapi.Bind(c, &in) {
		return
	}
	u, err := h.service.UpdateName(c.Request.Context(), Current(c), in.DisplayName)
	if err != nil {
		httpapi.Fail(c, err)
		return
	}
	httpapi.Data(c, 200, u)
}
