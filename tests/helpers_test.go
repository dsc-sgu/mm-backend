package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/network"
)

// Tests for helper functions
func TestInitBackend(t *testing.T) {
	ctx := context.Background()
	network, err := network.New(ctx)
	assert.Nil(t, err)
	_, _, _ = initPostgres(ctx, network)
	_, port, err := initBackend(ctx, network)
	if err != nil {
		t.Fatalf("initBackend error: %v", err)
	}
	t.Logf("Backend started on port: %v", port.Port())
}

func TestInitPostgres(t *testing.T) {
	ctx := context.Background()
	network, err := network.New(ctx)
	assert.Nil(t, err)
	_, port, err := initPostgres(ctx, network)
	if err != nil {
		t.Fatalf("initPostgres error: %v", err)
	}
	t.Logf("Postgres started on port: %v", port.Port())
}
