package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
)

func TestRegisterUser(t *testing.T) {
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)

	_, err = initPostgres(ctx, t, net)
	require.NoError(t, err)

	port, err := initBackend(ctx, t, net)
	require.NoError(t, err)

	userID := CreateTestUser(
		t,
		port,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)
}

func TestLoginUser(t *testing.T) {
	ctx := context.Background()

	net, err := network.New(ctx)
	require.NoError(t, err)

	_, err = initPostgres(ctx, t, net)
	require.NoError(t, err)

	_, err = initRedis(ctx, t, net)
	require.NoError(t, err)

	port, err := initBackend(ctx, t, net)
	require.NoError(t, err)

	userID := CreateTestUser(
		t,
		port,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	url := fmt.Sprintf("http://localhost:%s/api/v1/auth/login",
		port.Port(),
	)

	body, _ := json.Marshal(users.LoginModel{
		Email:    "test@email.com",
		Password: "password",
	})

	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var recievedSession users.LoginResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&recievedSession))

	require.Equal(t, userID, recievedSession.UserId)
}
