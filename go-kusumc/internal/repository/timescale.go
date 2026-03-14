package repository

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func InitDB(uri string) {
	config, err := pgxpool.ParseConfig(uri)
	if err != nil {
		log.Fatalf("❌ Invalid Database URI: %v", err)
	}

	// Performance Tuning
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnLifetime = 1 * time.Hour

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatalf("❌ Failed to connect to TimescaleDB: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("❌ Failed to ping TimescaleDB: %v", err)
	}
	
	DB = pool
	log.Println("✅ Connected to TimescaleDB")
}
