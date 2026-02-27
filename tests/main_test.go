package tests

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/docker/go-connections/nat"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go/network"
)

var (
	backendPort  nat.Port
	testPostgres *sql.DB
	testRedis    *redis.Client
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	net, err := network.New(ctx)
	if err != nil {
		log.Fatal("create network:", err)
	}
	defer func() {
		if err := net.Remove(ctx); err != nil {
			log.Print("remove network:", err)
		}
	}()

	pgContainer, pgPort, err := initPostgres(ctx, net)
	if err != nil {
		log.Fatal("start postgres:", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Print("terminate postgres:", err)
		}
	}()

	dsn := fmt.Sprintf(
		"host=127.0.0.1 port=%s user=postgres password=postgres dbname=postgres sslmode=disable",
		pgPort.Port(),
	)
	pgDB, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("connect to postgres:", err)
	}
	if err := pgDB.PingContext(ctx); err != nil {
		log.Fatal("ping postgres:", err)
	}

	testPostgres = pgDB
	defer func() {
		if err := testPostgres.Close(); err != nil {
			log.Print("close postgres:", err)
		}
	}()

	redisContainer, redisPort, err := initRedis(ctx, net)
	if err != nil {
		log.Fatal("start redis:", err)
	}
	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			log.Print("terminate redis:", err)
		}
	}()

	redisAddr := fmt.Sprintf("127.0.0.1:%s", redisPort.Port())
	redisDB := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := redisDB.Ping(ctx).Err(); err != nil {
		log.Fatal("ping redis:", err)
	}
	testRedis = redisDB
	defer func() {
		if err := testRedis.Close(); err != nil {
			log.Print("close redis:", err)
		}
	}()

	backendContainer, port, err := initBackend(ctx, net)
	if err != nil {
		log.Fatal("start backend:", err)
	}
	defer func() {
		if err := backendContainer.Terminate(ctx); err != nil {
			log.Print("terminate backend:", err)
		}
	}()

	backendPort = *port

	exitCode := m.Run()
	os.Exit(exitCode)
}
