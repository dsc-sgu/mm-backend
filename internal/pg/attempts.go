package pg

import (
	"database/sql"

	"github.com/google/uuid"
	"go.uber.org/zap"

	attempt "github.com/dsc-sgu/mm-backend/internal/attempts"
)

const GetAttemptsSql = `
	SELECT 
		a.id AS attempt_id, 
		a.user_id, 
		a.task_id, 
		att.state, 
		att.transition_at, 
		att.transition_data
	FROM attempt_transitions att
	JOIN attempts a ON att.attempt_id = a.id
	WHERE a.user_id = $1 AND a.task_id = $2
	ORDER BY att.transition_at ASC;
`

func (r *PGRepo) GetAttempts(userId, taskId uuid.UUID) ([]attempt.Attempt, error) {
	var attemptList []attempt.Attempt

	rows, err := r.db.Queryx(GetAttemptsSql, userId, taskId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			zap.L().Error("Failed to close rows", zap.Error(err))
		}
	}()

	for rows.Next() {
		var attempt attempt.Attempt
		if err := rows.StructScan(&attempt); err != nil {
			return nil, err
		}
		attemptList = append(attemptList, attempt)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return attemptList, nil
}
