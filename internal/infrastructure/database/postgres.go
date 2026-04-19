package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/killerquinn/referral-system-go/internal/config"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func NewPool(cfg *config.Config) (*Postgres, error) {
	ctx := context.Background()

	poolConfig, err := pgxpool.ParseConfig(cfg.Postgres.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to proccess dsn: %w", err)
	}
	poolConfig.MaxConns = int32(cfg.Postgres.MaxConnections)
	poolConfig.MinConns = int32(cfg.Postgres.MinConnections)
	poolConfig.MaxConnLifetime = cfg.Postgres.MaxConnectionLifetime
	poolConfig.MaxConnIdleTime = cfg.Postgres.MaxConnectionIdleLifetime
	poolConfig.HealthCheckPeriod = cfg.Postgres.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping pool: %w", err)
	}

	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) Close() error {
	if p.Pool != nil {
		fmt.Println("closing pool")
		p.Pool.Close()
	}
	fmt.Println("already closed")
	return nil
}

func (p *Postgres) HealthCheck(ctx context.Context) error {
	if p.Pool == nil {
		return fmt.Errorf("pool is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := p.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping pool: %w", err)
	}
	return nil
}
