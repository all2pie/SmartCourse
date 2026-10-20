//go:build integration

package testutil

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"gorm.io/gorm"

	"github.com/all2pie/go-smart-course/internal/platform/mongodb"
	"github.com/all2pie/go-smart-course/internal/platform/postgres"
	"github.com/all2pie/go-smart-course/internal/platform/redisdb"
)

// Images match docker-compose.yml.
const (
	postgresImage = "postgres:16-alpine"
	mongoImage    = "mongo:8"
	redisImage    = "redis:8-alpine"
)

// Postgres starts a database named after service with migrations/<service> applied.
func Postgres(t testing.TB, service string) *gorm.DB {
	t.Helper()
	ctx := context.Background()

	c, err := tcpostgres.Run(ctx, postgresImage,
		tcpostgres.WithDatabase(service),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("starting postgres: %v", err)
	}

	host, port := endpoint(t, c, "5432/tcp")
	db, err := postgres.Connect(postgres.Config{Host: host, Port: port, User: "postgres", Password: "postgres", Name: service})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range upMigrations(t, service, ".up.sql") {
		if err := db.Exec(string(m)).Error; err != nil {
			t.Fatalf("applying migration: %v", err)
		}
	}
	return db
}

// Mongo starts MongoDB and returns a database named after service with migrations/<service> applied.
func Mongo(t testing.TB, service string) *mongo.Database {
	t.Helper()
	ctx := context.Background()

	// MongoDB 8 refuses to start on Linux kernels >= 6.19 without this (see AGENTS.md).
	c, err := tcmongo.Run(ctx, mongoImage, testcontainers.WithEnv(map[string]string{"GLIBC_TUNABLES": "glibc.pthread.rseq=1"}))
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("starting mongo: %v", err)
	}

	uri, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	db, err := mongodb.Connect(ctx, mongodb.Config{URI: uri, Name: service})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Client().Disconnect(context.Background()) })

	// golang-migrate's Mongo files are JSON arrays of database commands.
	for _, m := range upMigrations(t, service, ".up.json") {
		var wrapped struct{ Commands []bson.D }
		if err := bson.UnmarshalExtJSON([]byte(`{"commands":`+string(m)+`}`), false, &wrapped); err != nil {
			t.Fatalf("parsing migration: %v", err)
		}
		for _, cmd := range wrapped.Commands {
			if err := db.RunCommand(ctx, cmd).Err(); err != nil {
				t.Fatalf("applying migration: %v", err)
			}
		}
	}
	return db
}

// Redis starts Redis and returns a connected client.
func Redis(t testing.TB) *redis.Client {
	t.Helper()
	ctx := context.Background()

	c, err := tcredis.Run(ctx, redisImage)
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("starting redis: %v", err)
	}

	host, port := endpoint(t, c, "6379/tcp")
	rdb, err := redisdb.Connect(ctx, redisdb.Config{Addr: net.JoinHostPort(host, strconv.Itoa(port))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func endpoint(t testing.TB, c testcontainers.Container, port string) (string, int) {
	t.Helper()
	hostPort, err := c.PortEndpoint(context.Background(), port, "")
	if err != nil {
		t.Fatal(err)
	}
	host, p, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		t.Fatal(err)
	}
	return host, n
}

// upMigrations returns the service's up migrations in version order.
func upMigrations(t testing.TB, service, suffix string) [][]byte {
	t.Helper()
	dir := filepath.Join(RepoRoot(t), "migrations", service)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var out [][]byte
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}
