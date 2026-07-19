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

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)
}

func TestGetCourseByID(t *testing.T) {
	clearDatabases(t)

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s",
		backendPort.Port(),
		courseID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var receivedCourse courses.Course
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&receivedCourse))

	require.Equal(t, courseID, receivedCourse.ID)
	require.Equal(t, "Test Course", receivedCourse.Name)
	require.Equal(t, disciplineID, receivedCourse.DisciplineID)
	require.NotNil(t, receivedCourse.ActiveSnapshotID)
	require.Equal(t, 1, receivedCourse.Version)
}

func TestGetPaginatedCourse(t *testing.T) {
	clearDatabases(t)

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	for i := range 5 {
		id := CreateTestCourse(
			t,
			&backendPort,
			&testUser,
			disciplineID,
			fmt.Sprintf("Course %d", i),
		)
		require.NotZero(t, id)
	}

	limit := 2
	lastID := uuid.Nil

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses?limit=%d&last_id=%s",
		backendPort.Port(),
		limit,
		lastID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := testUser.Client.Do(req)
	require.NoError(t, err)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var receivedCourses []*courses.Course
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&receivedCourses))

	require.Len(t, receivedCourses, limit)

	require.Equal(t, "Course 0", receivedCourses[0].Name)
	require.Equal(t, "Course 1", receivedCourses[1].Name)
}

func TestUpdateCourse(t *testing.T) {
	clearDatabases(t)

	testUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Test First Name",
		"Test Last Name",
		"Username",
		"test@email.com",
		"password",
	)
	require.NotZero(t, testUser.ID)

	disciplineID := CreateTestDiscipline(
		t,
		&backendPort,
		&testUser,
		"Test Discipline",
	)
	require.NotZero(t, disciplineID)

	courseID := CreateTestCourse(
		t,
		&backendPort,
		&testUser,
		disciplineID,
		"Test Course",
	)
	require.NotZero(t, courseID)

	url := fmt.Sprintf(
		"http://127.0.0.1:%s/api/v1/courses/%s",
		backendPort.Port(),
		courseID,
	)

	updatedName := "Updated Test Course"
	body, _ := json.Marshal(courses.UpdateCourse{
		OwnerID: &testUser.ID,
		Name:    &updatedName,
	})

	req, err := http.NewRequest(
		http.MethodPatch,
		url,
		bytes.NewBuffer(body),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testUser.Client.Do(req)
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
	require.NotNil(t, updatedCourse.ActiveSnapshotID)
	require.Equal(t, 1, updatedCourse.Version)
}

func TestCourseInviteWorkflow(t *testing.T) {
	clearDatabases(t)

	// Create a teacher user and a student user
	teacherTestUser := CreateAndLoginUser(
		t,
		&backendPort,
		"Teacher",
		"User",
		"teacher",
		"teacher@test.com",
		"password",
	)
	studentTestUser := CreateAndLoginUser(
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
		&teacherTestUser,
		"Test Discipline",
	)

	// Create a course as the teacher user
	courseID := CreateTestCourse(
		t,
		&backendPort,
		&teacherTestUser,
		disciplineID,
		"Test Course",
	)

	// Verify that the course creator is automatically set as a teacher
	role := GetRoleInCourse(t, &backendPort, &teacherTestUser, courseID)
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
			"http://127.0.0.1:%s/api/v1/course-invites",
			backendPort.Port(),
		)
		expiresAt := time.Now().Add(24 * time.Hour)
		inviteBody, _ := json.Marshal(courses.CreateInvite{
			CourseID:     courseID,
			ProvidedRole: courses.StudentRole,
			ExpiresAt:    &expiresAt,
		})

		req, err := http.NewRequest(
			http.MethodPost,
			inviteURL,
			bytes.NewBuffer(inviteBody),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := studentTestUser.Client.Do(req)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()

		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("Successfully create invite as teacher", func(t *testing.T) {
		inviteURL := fmt.Sprintf(
			"http://127.0.0.1:%s/api/v1/course-invites",
			backendPort.Port(),
		)
		expiresAt := time.Now().Add(24 * time.Hour)
		inviteBody, _ := json.Marshal(courses.CreateInvite{
			CourseID:     courseID,
			ProvidedRole: courses.StudentRole,
			ExpiresAt:    &expiresAt,
		})

		req, err := http.NewRequest(
			http.MethodPost,
			inviteURL,
			bytes.NewBuffer(inviteBody),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := teacherTestUser.Client.Do(req)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()

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
			"http://127.0.0.1:%s/api/v1/course-invites/%s",
			backendPort.Port(),
			inviteID,
		)

		req, err := http.NewRequest(http.MethodGet, detailsURL, nil)
		require.NoError(t, err)

		resp, err := studentTestUser.Client.Do(req) // Any authenticated user can get invite details
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()

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
			"http://127.0.0.1:%s/api/v1/course-invites/%s",
			backendPort.Port(),
			inviteID,
		)

		req, err := http.NewRequest(http.MethodPost, joinURL, nil)
		require.NoError(t, err)

		resp, err := studentTestUser.Client.Do(req)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify new role
		studentRole := GetRoleInCourse(t, &backendPort, &studentTestUser, courseID)
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
			"http://127.0.0.1:%s/api/v1/course-invites/%s",
			backendPort.Port(),
			inviteID,
		)

		req, err := http.NewRequest(http.MethodPost, joinURL, nil)
		require.NoError(t, err)

		resp, err := studentTestUser.Client.Do(req)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Error(err)
			}
		}()

		require.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}
