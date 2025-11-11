// Package tests should contain integration tests for the backend.
package tests

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
)

func initBackend(
	t *testing.T,
	net *testcontainers.DockerNetwork,
) (nat.Port, error) {
	// 1. initialize container request
	// 2. create request using GenericContainer
	// 3. call function which will clean up container after test
}

func initPostgres(
// 1. initialize container request
// 2. create request using GenericContainer
// 3. call function which will clean up container after test
) (nat.Port, error)

func sendRequest(
	t *testing.T,
	net *testcontainers.DockerNetwork,
	//
)

func readBodyToMap(rc io.ReadCloser) (map[string]any, error) {
	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}
