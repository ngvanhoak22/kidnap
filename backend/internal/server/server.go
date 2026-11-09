package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"journey-safety/backend/internal/auth"
	"journey-safety/backend/internal/config"
	"journey-safety/backend/internal/contacts"
	"journey-safety/backend/internal/platform/database"
	"journey-safety/backend/internal/platform/httpapi"
)

func Router(cfg config.Config, db *pgxpool.Pool, logger *slog.Logger) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, err
	}
	r.Use(httpapi.RequestMiddleware(logger, cfg.AllowedOrigins), httpapi.Recovery())
	r.NoRoute(func(c *gin.Context) { httpapi.Fail(c, httpapi.NotFound()) })
	r.GET("/health/live", func(c *gin.Context) { httpapi.Data(c, 200, gin.H{"status": "ok"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if !database.Ready(ctx, db) {
			httpapi.Fail(c, httpapi.NewError(503, "NOT_READY", "Dịch vụ chưa sẵn sàng."))
			return
		}
		httpapi.Data(c, 200, gin.H{"status": "ready"})
	})
	svc, err := auth.NewService(auth.NewRepository(db), cfg.SessionTTL)
	if err != nil {
		return nil, err
	}
	a := auth.NewHandler(svc, cfg)
	ct := contacts.NewHandler(contacts.NewService(contacts.NewRepository(db), cfg.InvitationTTL))
	v1 := r.Group("/api/v1")
	limiter := httpapi.NewLimiter(cfg.AuthRatePerMinute)
	v1.POST("/auth/register", limiter.Middleware(), a.Register)
	v1.POST("/auth/login", limiter.Middleware(), a.Login)
	private := v1.Group("")
	private.Use(a.RequireSession())
	private.GET("/auth/session", a.Session)
	private.POST("/auth/logout", a.Logout)
	private.GET("/auth/sessions", a.Sessions)
	private.DELETE("/auth/sessions", a.RevokeAll)
	private.DELETE("/auth/sessions/:id", a.Revoke)
	private.GET("/users/me", a.Me)
	private.PATCH("/users/me", a.UpdateMe)
	private.GET("/trusted-contacts", ct.List)
	private.GET("/trusted-contacts/trusted-by", ct.TrustedBy)
	private.DELETE("/trusted-contacts/:id", ct.Revoke)
	private.PATCH("/trusted-contacts/:id", ct.Update)
	private.POST("/trusted-contacts/invitations", ct.Invite)
	private.GET("/trusted-contacts/invitations", ct.Invitations)
	private.POST("/trusted-contacts/invitations/:id/accept", ct.Accept)
	private.POST("/trusted-contacts/invitations/:id/decline", ct.Decline)
	private.DELETE("/trusted-contacts/invitations/:id", ct.Cancel)
	return r, nil
}
