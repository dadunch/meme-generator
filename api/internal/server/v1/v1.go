// Package v1 memasang seluruh route API versi 1.
package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/nyxsr/meme-portal/api/internal/config"
	"github.com/nyxsr/meme-portal/api/internal/middleware"
	"github.com/nyxsr/meme-portal/api/internal/modules/auth"
	"github.com/nyxsr/meme-portal/api/internal/modules/meme"
	"github.com/nyxsr/meme-portal/api/internal/modules/subscription"
	"github.com/nyxsr/meme-portal/api/internal/platform/token"
)

// subscriptionGate mengadaptasi subscription.Service ke meme.Gate.
type subscriptionGate struct {
	sub  *subscription.Service
	pool *pgxpool.Pool
}

func (g subscriptionGate) CanCreate(ctx context.Context, userID string) error {
	tx, err := g.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Kunci advisory berbasis user ID selama transaksi untuk mencegah race condition (TKT-109)
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtext($1))`, userID).Scan(&locked); err != nil {
		return err
	}

	status, err := g.sub.Status(ctx, userID)
	if err != nil {
		return err
	}
	if !status.Subscribed {
		return meme.ErrNotSubscribedGate
	}
	if status.MaxMemes > 0 && status.MemesUsed >= status.MaxMemes {
		return meme.ErrNotSubscribedGate
	}

	_ = tx.Commit(ctx)
	return nil
}

func Mount(mux *http.ServeMux, pool *pgxpool.Pool, rdb *redis.Client, s3Client *s3.Client, tokens *token.Manager, cfg config.Config) {
	authHandler := auth.NewModule(pool, tokens)
	authMW := middleware.Auth(tokens)

	subHandler := subscription.NewModule(pool)
	subSvc := subscription.NewService(subscription.NewPostgresRepository(pool), subscription.MockGateway{})

	memeHandler := meme.NewModule(pool, s3Client, cfg.Storage, subscriptionGate{sub: subSvc, pool: pool})

	auth.RegisterRoutes(mux, authHandler, authMW)
	subscription.RegisterRoutes(mux, subHandler, authMW)
	meme.RegisterRoutes(mux, memeHandler, authMW)

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "code": 0, "message": "ok"})
	})
}
