package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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

func TestCourseInviteWorkflow(t *testing.T) {
	clearDatabases(t)

	// Create a teacher user and a student user
	teacherUser := CreateTestUser(
		t,
		&backendPort,
		"Teacher",
		"User",
		"teacher",
		"teacher@test.com",
		"password",
	)
	studentUser := CreateTestUser(
		t,
		&backendPort,
		"Student",
		"User",
		"student",
		"student@test.com",
		"password",
	)
	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		teacherUser,
		"Test Discipline",
	)

	// Create a course as the teacher user
	courseID := CreateTestCourse(
		t,
		&backendPort,
		teacherUser,
		disciplineID,
		"Test Course",
	)

	// Verify that the course creator is automatically set as a teacher
	role := GetRoleInCourse(t, &backendPort, teacherUser, courseID)
	require.NotNil(t, role, "Course creator should have a role")
	require.Equal(
		t,
		courses.TeacherRole,
		*role,
		"Course creator should be a teacher",
	)

	var inviteID uuid.UUID
	var invite courses.Invite

	// Run sub-tests for the invite workflow
	t.Run("Fail to create invite as non-teacher", func(t *testing.T) {
		inviteURL := fmt.Sprintf(
			"http://localhost:%s/api/v1/courses/invites?fake_user_id=%s",
			backendPort.Port(),
			studentUser,
		)
		inviteBody, _ := json.Marshal(courses.CreateInvite{
			CourseID:     courseID,
			ProvidedRole: courses.StudentRole,
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		})

		req, err := http.NewRequest(
			http.MethodPost,
			inviteURL,
			bytes.NewBuffer(inviteBody),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("Successfully create invite as teacher", func(t *testing.T) {
		inviteURL := fmt.Sprintf(
			"http://localhost:%s/api/v1/courses/invites?fake_user_id=%s",
			backendPort.Port(),
			teacherUser,
		)
		inviteBody, _ := json.Marshal(courses.CreateInvite{
			CourseID:     courseID,
			ProvidedRole: courses.StudentRole,
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		})

		req, err := http.NewRequest(
			http.MethodPost,
			inviteURL,
			bytes.NewBuffer(inviteBody),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&invite))
		require.NotZero(t, invite.ID)
		inviteID = invite.ID // Save for next steps
	})

	t.Run("Get invite details", func(t *testing.T) {
		require.NotZero(
			t,
			inviteID,
			"inviteID should be set from previous test",
		)
		detailsURL := fmt.Sprintf(
			"http://localhost:%s/api/v1/courses/invites/%s?fake_user_id=%s",
			backendPort.Port(),
			inviteID,
			studentUser,
		)

		req, err := http.NewRequest(http.MethodGet, detailsURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var details courses.InviteDetails
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&details))
		require.Equal(t, "Test Course", details.CourseName)
		require.Equal(t, courses.StudentRole, details.ProvidedRole)
	})

	t.Run("Join course with invite", func(t *testing.T) {
		require.NotZero(
			t,
			inviteID,
			"inviteID should be set from previous test",
		)
		joinURL := fmt.Sprintf(
			"http://localhost:%s/api/v1/courses/invites/%s?fake_user_id=%s",
			backendPort.Port(),
			inviteID,
			studentUser,
		)

		req, err := http.NewRequest(http.MethodPost, joinURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify new role
		studentRole := GetRoleInCourse(t, &backendPort, studentUser, courseID)
		require.NotNil(
			t,
			studentRole,
			"Student should now have a role in the course",
		)
		require.Equal(t, courses.StudentRole, *studentRole)
	})

	t.Run("Fail to join course again", func(t *testing.T) {
		require.NotZero(
			t,
			inviteID,
			"inviteID should be set from previous test",
		)
		joinURL := fmt.Sprintf(
			"http://localhost:%s/api/v1/courses/invites/%s?fake_user_id=%s",
			backendPort.Port(),
			inviteID,
			studentUser,
		)

		req, err := http.NewRequest(http.MethodPost, joinURL, nil)
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}
