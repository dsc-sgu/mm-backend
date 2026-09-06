package tasks

import (
	"context"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/courses/membership"
	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type RepositoryStore interface {
	UpdateTemplate(taskGroupID uuid.UUID, files []pkggit.FileInfo) error
	WritePatterns(repoID pkggit.RepoID, patterns map[string][]string) error
}

// MemberChecker resolves a course membership, the same gate course-editing
// uses (membership.Service.CheckMember), so task groups share one
// authorization source of truth with the rest of the course.
type MemberChecker interface {
	CheckMember(ctx context.Context, userID, courseID uuid.UUID) (*membership.Member, error)
}

type Service struct {
	Repo
	repositories RepositoryStore
	members      MemberChecker
}

func NewService(repo Repo, repositories RepositoryStore, members MemberChecker) *Service {
	return &Service{Repo: repo, repositories: repositories, members: members}
}

// requireMember ensures userID is any active member (student or teacher) of courseID.
func (s *Service) requireMember(ctx context.Context, userID, courseID uuid.UUID) error {
	_, err := s.members.CheckMember(ctx, userID, courseID)
	return err
}

// requireTeacher ensures userID is an active teacher of courseID.
func (s *Service) requireTeacher(ctx context.Context, userID, courseID uuid.UUID) error {
	member, err := s.members.CheckMember(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if member.Role != membership.TeacherRole {
		return membership.ErrPermissionDenied
	}
	return nil
}

// CreateTaskGroup requires the caller to be an active teacher of the course
// the group is being created in.
func (s *Service) CreateTaskGroup(ctx context.Context, userID uuid.UUID, model *CreateTaskGroup) (*TaskGroup, error) {
	if err := s.requireTeacher(ctx, userID, model.CourseID); err != nil {
		return nil, err
	}
	return s.Repo.CreateTaskGroup(ctx, model)
}

// GetTaskGroup returns a task group with its tasks for any active course
// member. Returns (nil, nil) if the group does not exist. The caller sees
// their own in-progress draft's tasks if they hold the course's edit lock,
// otherwise the course's active (published) tasks.
func (s *Service) GetTaskGroup(ctx context.Context, userID, sessionID, groupID uuid.UUID) (*TaskGroupWithTasks, error) {
	tg, err := s.Repo.GetTaskGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if tg == nil {
		return nil, nil
	}
	if err := s.requireMember(ctx, userID, tg.CourseID); err != nil {
		return nil, err
	}

	viewSnapshotID, err := s.Repo.ResolveViewSnapshot(ctx, tg.CourseID, userID, sessionID)
	if err != nil {
		return nil, err
	}
	taskList, err := s.Repo.GetTasks(ctx, groupID, viewSnapshotID)
	if err != nil {
		return nil, err
	}
	return &TaskGroupWithTasks{TaskGroup: *tg, Tasks: taskList}, nil
}

// GetTasks lists a task group's tasks for any active course member, scoped
// the same way GetTaskGroup is (own draft if editing, else the active snapshot).
func (s *Service) GetTasks(ctx context.Context, userID, sessionID, groupID uuid.UUID) ([]*Task, error) {
	courseID, err := s.Repo.GetCourseIDByTaskGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if err := s.requireMember(ctx, userID, courseID); err != nil {
		return nil, err
	}
	viewSnapshotID, err := s.Repo.ResolveViewSnapshot(ctx, courseID, userID, sessionID)
	if err != nil {
		return nil, err
	}
	return s.Repo.GetTasks(ctx, groupID, viewSnapshotID)
}

// UpdateTaskGroup requires the caller to be an active teacher of the group's course.
func (s *Service) UpdateTaskGroup(ctx context.Context, userID, groupID uuid.UUID, update *UpdateTaskGroup) (*TaskGroup, error) {
	courseID, err := s.Repo.GetCourseIDByTaskGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return nil, err
	}
	return s.Repo.UpdateTaskGroup(ctx, groupID, update)
}

// DeleteTaskGroup requires the caller to be an active teacher of the group's course.
func (s *Service) DeleteTaskGroup(ctx context.Context, userID, groupID uuid.UUID) error {
	courseID, err := s.Repo.GetCourseIDByTaskGroup(ctx, groupID)
	if err != nil {
		return err
	}
	if err := s.requireTeacher(ctx, userID, courseID); err != nil {
		return err
	}
	return s.Repo.DeleteTaskGroup(ctx, groupID)
}

// UploadTemplate requires the caller to be an active teacher of the group's course.
func (s *Service) UploadTemplate(ctx context.Context, userID, groupID uuid.UUID, zipData []byte) error {
	tg, err := s.Repo.GetTaskGroupByID(ctx, groupID)
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

func (s *Service) RefreshRepositoryPatterns(ctx context.Context, repoID pkggit.RepoID) error {
	patterns, err := s.GetTaskPatterns(ctx, repoID.TaskGroupID)
	if err != nil {
		return err
	}
	return s.repositories.WritePatterns(repoID, patterns)
}
