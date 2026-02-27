package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/auth/users"
)

func TestRegisterUser(t *testing.T) {
	clearDatabases(t)

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)
}

func TestLoginUser(t *testing.T) {
	clearDatabases(t)

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, userID)

	url := fmt.Sprintf("http://127.0.0.1:%s/api/v1/auth/login",
		backendPort.Port(),
	)

	body, _ := json.Marshal(users.LoginUser{
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

	require.Equal(t, userID, recievedSession.UserID)

	cookies := resp.Cookies()
	require.Len(t, cookies, 1)

	sessionCookie := cookies[0]
	require.Equal(t, "SESSION_ID", sessionCookie.Name)
	require.NotEmpty(t, sessionCookie.Value)
	require.Equal(t, "/", sessionCookie.Path)
	require.True(t, sessionCookie.HttpOnly)
}
