package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

// Tests for courses
func TestCreateCourse(t *testing.T) {
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

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)
}

func TestGetCourseByID(t *testing.T) {
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

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)

	url := fmt.Sprintf(
		"http://localhost:%s/api/v1/courses/%s?fake_user_id=%s",
		backendPort.Port(),
		courseID,
		userID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var recievedCourse courses.Course
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&recievedCourse))

	require.Equal(t, courseID, recievedCourse.ID)
	require.Equal(t, "Test Course", recievedCourse.Name)
	require.Equal(t, disciplineID, recievedCourse.DisciplineID)
}

func TestGetPaginatedCourse(t *testing.T) {
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

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	for i := range 5 {
		id := CreateTestCourse(
			t,
			&backendPort,
			userID,
			disciplineID,
			fmt.Sprintf("Course %d", i),
		)
		require.NotZero(t, id)
	}

	limit := 2
	lastID := uuid.Nil

	url := fmt.Sprintf(
		"http://localhost:%s/api/v1/courses?limit=%d&last_id=%s&fake_user_id=%s",
		backendPort.Port(),
		limit,
		lastID,
		userID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var recievedCourses []*courses.Course
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&recievedCourses))

	require.Len(t, recievedCourses, limit)

	require.Equal(t, "Course 0", recievedCourses[0].Name)
	require.Equal(t, "Course 1", recievedCourses[1].Name)
}

func TestUpdateCourse(t *testing.T) {
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

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		userID,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		userID,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)

	url := fmt.Sprintf(
		"http://localhost:%s/api/v1/courses/%s?fake_user_id=%s",
		backendPort.Port(),
		courseID,
		userID,
	)

	body, _ := json.Marshal(courses.UpdateCourse{
		OwnerID: userID,
		Name:    "Updated Test Course",
	})

	req, err := http.NewRequest(
		http.MethodPatch,
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

	var updatedCourse courses.Course
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&updatedCourse))

	require.Equal(t, courseID, updatedCourse.ID)
	require.Equal(t, "Updated Test Course", updatedCourse.Name)
	require.Equal(t, disciplineID, updatedCourse.DisciplineID)
}
