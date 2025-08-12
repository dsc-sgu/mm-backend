package attempt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type AttemptRepository interface {
	CreateAttempt(ctx context.Context, req *MakeAttempt) (*Attempt, error)
	GetAttempt(ctx context.Context, attemptID uuid.UUID) (*AttemptResponse, error)
	UpdatedAttempt(ctx context.Context, attemptUpdate AttemptUpdate) (*Attempt, error)
	DeleteAttempt(ctx context.Context, attemptID uuid.UUID) error

	GetAllAttempts(ctx context.Context, participantID uuid.UUID, courseID uuid.UUID, taskID uuid.UUID) ([]Attempt, error)
	// TODO(xseniva): define rewiewAttempt
	GradeAttempt(ctx context.Context, attempt *Attempt) (*rewiewAttempt, error)
}

type attemptRepository struct {
	db        *sqlx.DB
	manager   RepoManager
	fileStore FileStorage
}

func NewAttemptRepository(db *sqlx.DB, manager RepoManager, fileStore FileStorage) AttemptRepository {
	return &attemptRepository{
		db:        db,
		manager:   manager,
		fileStore: fileStore,
	}
}

const addAttemptToAttempts = `
    INSERT INTO attempts (id, user_id, task_id, course_id)
    VALUES (:id, :participiant_id, :task_id, :course_id)
    RETURNING id
`

const addAttemptToAttemptTransit = `
    INSERT INTO attempt_transitions (attempt_id, state, transition_at, transition_data)
    VALUES (:attempt_id, :state, :transition_at, :transition_data)
    RETURNING id
`

func execWithRowsCheck[T any](tx *sqlx.Tx, query string, args T) (int64, error) {
	res, err := tx.NamedExec(query, args)
	if err != nil {
		return 0, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (a *attemptRepository) CreateAttempt(ctx context.Context, req *MakeAttempt) (*Attempt, error) {
	attempt, err := a.manager.MakeAttempt(req.RepoID, req.Files)

	if err != nil {
		err := fmt.Errorf("attempt creation: %w", err)
		zap.S().Error(err)
		return nil, err
	}

	AttemptDB := AttemptDB{
		Id:       attempt.Id,
		UserID:   req.ParticipantID,
		TaskID:   req.TaskId,
		CourseID: req.CourseId,
	}

	AttemptTransitDB := AttemptTransitDB{
		Id:           attempt.Id,
		State:        AttemptStatusSubmited,
		TransitionAt: time.Now(),
	}

	tx, err := a.db.Beginx()
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	rows, err := execWithRowsCheck(tx, addAttemptToAttempts, AttemptDB)
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, errors.New("no rows affected in addAttemptToAttempts")
	}

	rows, err = execWithRowsCheck(tx, addAttemptToAttemptTransit, AttemptTransitDB)
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		return nil, errors.New("no rows affected in addAttemptToAttemptTransit")
	}

	return &attempt, nil
}

const getAttempt = `
   SELECT attempt.id, attempt_transitions.state, 
	 attempt_transitions.transition_at, attempt_transitions.transition_data
	 FROM attempt
	 JOIN attempt_transitions ON attempt_transitions.attempt_id = attempt.id
	 WHERE attempt.id= $1 
	 ORDER BY attempt.id, attempt_transitions.transition_at DESC
	 LIMIT 1
`

func (a *attemptRepository) GetAttempt(ctx context.Context, attemptID uuid.UUID) (*AttemptResponse, error) {
	var attempt AttemptTransitDB
	attemptData, err := a.manager.GetAttemptData(attemptID)
	err = a.db.GetContext(ctx, &attempt, getAttempt)
	if err != nil {
		return nil, err
	}
	attemptResp := AttemptResponse{
		AttemptTransitDB: attempt,
		AttemptDetails:   attemptData,
	}
	return &attemptResp, nil
}

const getAllAttempts = `
  SELECT attempt.id, attempt.user_id, 
	 attempt.task_id, attempt_transitions.state, 
	 attempt_transitions.transition_at, attempt_transitions.transition_data
	 FROM attempt
	 JOIN attempt_transitions ON attempt_transitions.attempt_id = attempt.id
	 WHERE attempt.user_id = $1 AND attempt.task_id = $2 AND attempt.course_id = $3
`

func (a *attemptRepository) GetAllAttempts(ctx context.Context, participantID uuid.UUID, courseID uuid.UUID, taskID uuid.UUID) ([]Attempt, error) {
	var attemptList []Attempt
	err := a.db.SelectContext(ctx, &attemptList, getAllAttempts, participantID, taskID, courseID)
	if err != nil {
		return nil, err
	}
	return attemptList, nil
}

func (a *attemptRepository) GradeAttempt(ctx context.Context, attempt *Attempt) (*rewiewAttempt, error) {
	AttemptTransitDB := AttemptTransitDB{
		Id:             attempt.Id,
		State:          AttemptStatusGraded,
		TransitionAt:   time.Now(),
		TransitionData: json.RawMessage(unpredictable_for_now),
	}

	rows, err := a.db.NamedQuery(addAttemptToAttemptTransit, AttemptTransitDB)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			err = fmt.Errorf("transaction error: %w", err)
			zap.S().Error(err)
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&AttemptTransitDB.Id); err != nil {
			return nil, err
		}
	}
	return &rewiewAttempt{}, nil
}

func (a *attemptRepository) UpdatedAttempt(ctx context.Context, attemptUpdate AttemptUpdate) (*Attempt, error) {
	id := attemptUpdate.Id
	// TODO(xseniva): add method to get FileDescriptor
	desc := FileDescriptor(id.String())

	err := a.fileStore.RemoveFile(desc)
	if err != nil {
		return err
	}

	fileInfo, err := a.fileStore.StoreFile(attemptUpdate.files)
	if err != nil {
		return err
	}

	attempt, err := a.manager.MakeAttempt(attemptUpdate.repoID, fileInfo)
	return attempt, nil
}

const checkStatus = `
  SELECT state
  FROM attempt_transitions
  WHERE attempt_id = $1
`

// TODO(xseniva): check cascade delete in bd
const deleteAttempt = `
  DELETE FROM attempt 
	WHERE id = :attempt_id;
`

func (a *attemptRepository) DeleteAttempt(ctx context.Context, attemptID uuid.UUID) error {
	var state string
	err := a.db.GetContext(ctx, &state, checkStatus, attemptID)
	if err != nil {
		return err
	}
	if AttemptState(state) == AttemptStatusGraded {
		err = fmt.Errorf("deleted error: the attempt was graded")
		zap.S().Error(err)
		return err
	}

	_, err = a.db.NamedExecContext(ctx, deleteAttempt, attemptID)
	if err != nil {
		return err
	}

	desc := FileDescriptor(attemptID.String())
	err = a.fileStore.RemoveFile(desc)
	if err != nil {
		err = fmt.Errorf("deleted error: RemoveFile: %w", err)
		zap.S().Error(err)
		return err
	}
	return nil
}
