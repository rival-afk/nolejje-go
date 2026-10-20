package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var (
	Pool        *pgxpool.Pool
	Ctx, Cancel = context.WithCancel(context.Background())
)

func DbURL() string {
	godotenv.Load()
	dbURL := os.Getenv("db_url")
	if dbURL == "" {
		dbURL = "postgres://postgres@localhost/nolejje"
	}
	return dbURL
}

func CreatePool() error {
	cfg, err := pgxpool.ParseConfig(DbURL())
	if err != nil {
		return err
	}
	Pool, err = pgxpool.NewWithConfig(Ctx, cfg)
	if err != nil {
		return err
	}
	if err = Pool.Ping(Ctx); err != nil {
		return err
	}
	log.Println("подключение к базе данных успешно, подключений в пуле:", Pool.Stat().MaxConns())
	return nil
}

func ClosePool() {
	if Pool != nil {
		Pool.Close()
	}
	Cancel()
}
