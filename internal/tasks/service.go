package tasks

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type RepositoryStore interface {
	UpdateTemplate(taskGroupID uuid.UUID, files []pkggit.FileInfo) error
}

// MemberChecker resolves a course membership, the same gate course-editing
// uses (membership.Service.CheckMember), so task groups share one
// authorization source of truth with the rest of the course.
type MemberChecker interface {
	CheckMember(
		ctx context.Context,
		userID, courseID uuid.UUID,
	) (*membership.Member, error)
}

type Service struct {
	repo         Repo
	repositories RepositoryStore
	members      MemberChecker
}

func NewService(
	repo Repo,
	repositories RepositoryStore,
	members MemberChecker,
) *Service {
	return &Service{repo: repo, repositories: repositories, members: members}
}

var (
	ErrTaskGroupNotFound  = errors.New("task group not found")
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskGroupNameTaken = errors.New(
		"a task group with this name already exists in the course",
	)
)

// requireTeacher ensures userID is an active teacher of courseID.
func (s *Service) requireTeacher(
	ctx context.Context,
	userID, courseID uuid.UUID,
) error {
	member, err := s.members.CheckMember(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if member.Role != membership.TeacherRole {
		return membership.ErrPermissionDenied
	}
	return nil
}

// viewer resolves whose drafts a reader of courseID sees: their own if they
// are a teacher, nobody's (uuid.Nil) if they are a student.
func (s *Service) viewer(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (uuid.UUID, error) {
	member, err := s.members.CheckMember(ctx, userID, courseID)
	if err != nil {
		return uuid.Nil, err
	}
	if member.Role == membership.TeacherRole {
		return userID, nil
	}
	return uuid.Nil, nil
}

// GetTaskGroup returns a task group with its tasks for any active course
// member. Returns (nil, nil) if the group does not exist. A teacher sees
// the tasks of their own draft over the published ones, a student the
// published tasks.
func (s *Service) GetTaskGroup(
	ctx context.Context,
	userID, groupID uuid.UUID,
) (*TaskGroupWithTasks, error) {
	tg, err := s.repo.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if tg == nil {
		return nil, nil
	}
	viewerID, err := s.viewer(ctx, userID, tg.CourseID)
	if err != nil {
		return nil, err
	}
	taskList, err := s.repo.GetTasks(ctx, groupID, viewerID)
	if err != nil {
		return nil, err
	}
	return &TaskGroupWithTasks{TaskGroup: *tg, Tasks: taskList}, nil
}

// GetTasks lists a task group's tasks for any active course member, scoped
// the same way GetTaskGroup is.
func (s *Service) GetTasks(
	ctx context.Context,
	userID, groupID uuid.UUID,
) ([]*Task, error) {
	courseID, err := s.repo.GetCourseIDByTaskGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	viewerID, err := s.viewer(ctx, userID, courseID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetTasks(ctx, groupID, viewerID)
}

// UpdateTaskGroup requires the caller to be an active teacher of the group's course.
func (s *Service) UpdateTaskGroup(
	ctx context.Context,
	userID, groupID uuid.UUID,
	update *UpdateTaskGroup,
) (*TaskGroup, error) {
	courseID, err := s.repo.GetCourseIDByTaskGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.repo.UpdateTaskGroup(ctx, groupID, update)
}

// UpdateTask requires the caller to be an active teacher of the task's course.
// A task lives outside block versions, so the change applies at once.
func (s *Service) UpdateTask(
	ctx context.Context,
	userID, taskID uuid.UUID,
	update *TaskUpdate,
) (*Task, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	courseID, err := s.repo.GetCourseIDByTaskGroup(ctx, task.TaskGroupID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.repo.UpdateTask(ctx, taskID, update)
}

// UploadTemplate requires the caller to be an active teacher of the group's
// course. The template repository is not versioned: the last upload wins.
func (s *Service) UploadTemplate(
	ctx context.Context,
	userID, groupID uuid.UUID,
	zipData []byte,
) error {
	tg, err := s.repo.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return err
	}
	if tg == nil {
		return ErrTaskGroupNotFound
	}
	if err := s.requireTeacher(ctx, userID, tg.CourseID); err != nil {
		return err
	}

	files, err := pkggit.UnzipFiles(zipData)
	if err != nil {
		return err
	}

	return s.repositories.UpdateTemplate(tg.ID, files)
}

func (s *Service) GetTaskGroupIDByName(
	ctx context.Context,
	name string,
	courseID uuid.UUID,
) (uuid.UUID, error) {
	return s.repo.GetTaskGroupIDByName(ctx, name, courseID)
}

func (s *Service) GetTaskByName(
	ctx context.Context,
	taskGroupID uuid.UUID,
	name string,
) (uuid.UUID, error) {
	return s.repo.GetTaskByName(ctx, taskGroupID, name)
}

func (s *Service) GetTaskPatternsByTaskID(
	ctx context.Context,
	taskID uuid.UUID,
) ([]string, error) {
	return s.repo.GetTaskPatternsByTaskID(ctx, taskID)
}
