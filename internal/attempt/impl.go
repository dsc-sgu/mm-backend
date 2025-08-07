package attempt

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type AttemptRepository interface {
	CreateAttempt(ctx context.Context, req *MakeAttempt) (*Attempt, error)
	GradeAttempt(ctx context.Context, attempt *Attempt) error // Allows change State in attempt_transition table to Graded
	GetLastAttempt(ctx context.Context, participantID uuid.UUID, courseID uuid.UUID, taskID uuid.UUID) (*Attempt, error)
	GetAllAttempts(ctx context.Context, participantID uuid.UUID, courseID uuid.UUID, taskID uuid.UUID) ([]Attempt, error)

	// ReviewAttempt(ctx context.Context, req ReviewAttemptRequest) (*models.Attempt, error)
	// DeleteAttempt(attemptID uuid.UUID) error
}

type attemptRepository struct {
	db      *sqlx.DB
	manager RepoManager
	logger  *zap.Logger
}

func NewAttemptRepository(db *sqlx.DB, manager RepoManager, logger *zap.Logger) AttemptRepository {
	return &attemptRepository{
		db:      db,
		manager: manager,
		logger:  logger,
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
		a.logger.Error("incorrect return from MakeAttempt")
		return nil, errors.New("incorrect return from MakeAttempt")
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
		a.logger.Error("no rows affected in attempts")
		return nil, errors.New("no rows affected in addAttemptToAttempts")
	}

	rows, err = execWithRowsCheck(tx, addAttemptToAttemptTransit, AttemptTransitDB)
	if err != nil {
		return nil, err
	}
	if rows == 0 {
		a.logger.Error("no rows affected in AttemptTransition")
		return nil, errors.New("no rows affected in addAttemptToAttemptTransit")
	}

	return &attempt, nil
}

func (a *attemptRepository) GradeAttempt(ctx context.Context, attempt *Attempt) error {
	AttemptTransitDB := AttemptTransitDB{
		Id:           attempt.Id,
		State:        AttemptStatusGraded,
		TransitionAt: time.Now(),
	}

	rows, err := a.db.NamedQuery(addAttemptToAttemptTransit, AttemptTransitDB)
	if err != nil {
		return err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			a.logger.Error(err.Error())
		}
	}()

	if rows.Next() {
		if err := rows.Scan(&AttemptTransitDB.Id); err != nil {
			return err
		}
	}

	return nil
}

const getLastAttempt = `
   SELECT attempt.id, attempt.user_id, 
	 attempt.task_id, attempt_transitions.state, 
	 attempt_transitions.transition_at, attempt_transitions.transition_data
	 FROM attempt
	 JOIN attempt_transitions ON attempt_transitions.attempt_id = attempt.id
	 WHERE attempt.user_id = $1 AND attempt.task_id = $2 AND attempt.course_id = $3
	 ORDER BY attempt.id, attempt_transitions.transition_at DESC
	 LIMIT 1
`

func (a *attemptRepository) GetLastAttempt(ctx context.Context, participantID uuid.UUID, courseID uuid.UUID, taskID uuid.UUID) (*Attempt, error) {
	var attempt Attempt
	err := a.db.GetContext(ctx, &attempt, getLastAttempt, participantID, taskID, courseID)
	if err != nil {
		return nil, err
	}
	return &attempt, nil
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

// func (a *attemptRepository) DeleteAttempt(id uuid.UUID) error {
// 	return nil
// }
