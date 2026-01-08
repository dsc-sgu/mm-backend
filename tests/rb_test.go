package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/network"
)

func TestCreateBlock(t *testing.T) {
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

	disciplineID := CreateTestDiscipline(
		t,
		port,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		port,
		userID,
		disciplineID,
		"Test Course",
	)

	blockID := CreateTestBlock(
		t,
		port,
		userID,
		courseID,
	)

	require.NotZero(t, blockID)
}
