package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
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

	email := "test@email.com"
	password := "password"

	userID := CreateTestUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		email,
		password,
	)
	require.NotZero(t, userID)

	testUser := LoginUser(t, &backendPort, email, password)
	require.Equal(t, userID, testUser.ID)
	require.NotNil(t, testUser.Client)
	require.NotZero(t, testUser.SessionID)
}
