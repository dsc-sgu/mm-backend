package attempt

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

type Service struct {
	repo    Repo
	git     GitManager
	tasks   TaskReader
	courses CourseReader
}

func NewService(
	repo Repo,
	git GitManager,
	tasks TaskReader,
	courses CourseReader,
) *Service {
	return &Service{
		repo:    repo,
		git:     git,
		tasks:   tasks,
		courses: courses,
	}
}

var (
	// ErrPermissionDenied is returned when a participant may not touch a course's repositories.
	ErrPermissionDenied = errors.New("permission denied")
	// ErrDifferentUsers is returned when a diff is requested between two attempts belonging to different users.
	ErrDifferentUsers = errors.New("attempts belong to different users")
	// ErrDifferentTasks is returned when a diff is requested between two attempts at different tasks.
	ErrDifferentTasks = errors.New("attempts belong to different tasks")
	// ErrPatternMismatch is returned when an uploaded archive contains none
	// of the files a task requires.
	ErrPatternMismatch = errors.New("no uploaded files match required patterns")
	// ErrEmptyArchive is returned when an uploaded archive contains no files.
	ErrEmptyArchive = errors.New("uploaded archive contains no files")
)

// GetAttempts lists a participant's attempts at a task. The caller must be
// that participant themself or an active teacher of the task's course.
func (s *Service) GetAttempts(
	ctx context.Context,
	callerID, taskID, participantID uuid.UUID,
) ([]Attempt, error) {
	if err := s.assertViewer(ctx, callerID, participantID, taskID); err != nil {
		return nil, err
	}
	return s.repo.GetAttempts(taskID, participantID)
}

// assertViewer authorizes callerID to view ownerID's data for a task: either
// they are the same person, or callerID is an active teacher of the course
// the task belongs to.
func (s *Service) assertViewer(
	ctx context.Context,
	callerID, ownerID, taskID uuid.UUID,
) error {
	if callerID == ownerID {
		return nil
	}
	courseID, err := s.repo.GetTaskCourseID(ctx, taskID)
	if err != nil {
		return err
	}
	isTeacher, err := s.courses.IsCourseTeacher(ctx, callerID, courseID)
	if err != nil {
		return fmt.Errorf("check course teacher: %w", err)
	}
	if !isTeacher {
		return ErrPermissionDenied
	}
	return nil
}

// assertMember is the single authorization gate for repository access; the HTTP
// and the SSH entry points both go through it.
func (s *Service) assertMember(
	ctx context.Context,
	participantID, courseID uuid.UUID,
) error {
	member, err := s.courses.IsCourseMember(ctx, participantID, courseID)
	if err != nil {
		return fmt.Errorf("check course membership: %w", err)
	}
	if !member {
		return ErrPermissionDenied
	}
	return nil
}

// repoForTask resolves the repository holding a participant's attempts at a task.
// The course comes from the task itself, so a caller cannot point an attempt at a
// course the task does not belong to.
func (s *Service) repoForTask(
	ctx context.Context,
	taskID, participantID uuid.UUID,
) (pkggit.RepoID, error) {
	id, err := s.repo.RepoForTask(ctx, taskID, participantID)
	if err != nil {
		return pkggit.RepoID{}, err
	}
	if err := s.assertMember(ctx, id.ParticipantID, id.CourseID); err != nil {
		return pkggit.RepoID{}, err
	}
	return id, nil
}

// Pushes files from added zip to a repo and makes an attempt by those rules: if zip is empty or all files from it dont match any patterns then user gets error and PushAttempt falls. Files from existing user repo get deleted if they dont match patterns. All files from zip get to repo.
func (s *Service) PushAttempt(
	ctx context.Context,
	taskID, participantID uuid.UUID,
	zipData []byte,
) (string, error) {
	id, err := s.repoForTask(ctx, taskID, participantID)
	if err != nil {
		return "", err
	}
	files, err := pkggit.UnzipFiles(zipData)
	if err != nil {
		return "", fmt.Errorf("extract zip: %w", err)
	}
	if len(files) == 0 {
		return "", ErrEmptyArchive
	}
	patterns, err := s.tasks.GetTaskPatternsByTaskID(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("get task patterns: %w", err)
	}
	compiledPatterns := pkggit.CompilePatterns(patterns)
	if len(patterns) > 0 {
		matched := false
		for _, file := range files {
			if compiledPatterns.MatchAny(file.FileName) {
				matched = true
				break
			}
		}
		if !matched {
			return "", fmt.Errorf("%w (%v)", ErrPatternMismatch, patterns)
		}
	}
	unlock := s.git.Lock(id)
	defer unlock()
	if err := s.git.EnsureRepo(id); err != nil {
		return "", fmt.Errorf("init repository: %w", err)
	}

	existing, err := s.git.ListFiles(id)
	if err != nil {
		return "", fmt.Errorf("list current files: %w", err)
	}
	touched := make(map[string]bool, len(files))
	for _, file := range files {
		touched[file.FileName] = true
	}
	// The repository is shared by the whole task group, so only this task's
	// files (the ones matching its patterns) are replaced by a resubmission.
	var remove []string
	for _, path := range existing {
		if !touched[path] && compiledPatterns.MatchAny(path) {
			remove = append(remove, path)
		}
	}

	hash, err := s.git.CommitFiles(s.git.RepoPath(id), files, remove, "web attempt")
	if err != nil {
		return "", err
	}
	if err := s.repo.SaveAttempt(id, taskID, hash); err != nil {
		return "", fmt.Errorf("save attempt: %w", err)
	}
	return hash, nil
}

func (s *Service) GetDiff(
	ctx context.Context,
	callerID, id1, id2 uuid.UUID,
) ([]pkggit.ChangedFile, error) {
	one, err := s.repo.GetAttemptCommitInfo(id1)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	two, err := s.repo.GetAttemptCommitInfo(id2)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	if one.UserID != two.UserID {
		return nil, ErrDifferentUsers
	}
	if one.TaskID != two.TaskID {
		return nil, ErrDifferentTasks
	}
	if callerID != one.UserID {
		isTeacher, err := s.courses.IsCourseTeacher(ctx, callerID, one.CourseID)
		if err != nil {
			return nil, fmt.Errorf("check course teacher: %w", err)
		}
		if !isTeacher {
			return nil, ErrPermissionDenied
		}
	}
	patterns, _ := s.tasks.GetTaskPatternsByTaskID(ctx, one.TaskID)
	id := pkggit.RepoID{
		CourseID:      one.CourseID,
		TaskGroupID:   one.TaskGroupID,
		ParticipantID: one.UserID,
	}
	// A task with no patterns leaves every file in the diff visible — that's
	// a nil include, not an always-false one, so it must not be built from
	// an empty CompiledPatterns (whose MatchAny is always false).
	var include func(string) bool
	if len(patterns) > 0 {
		include = pkggit.CompilePatterns(patterns).MatchAny
	}
	return s.git.Diff(id, one.CommitHash, two.CommitHash, include)
}
