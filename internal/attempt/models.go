package attempt

import (
	"time"

	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/git"
)

/*
Варианты работы с попытками:
- Студент отправил попытку с файлом через форму
- Студент отправил попытку с файлом через CLI
- Преподаватель решил оценить попытку
*/

// AttemptStatus adds table with Users Git Data to DB
type AttemptStatus string

const (
	AttemptStatusSent     AttemptStatus = "sent"
	AttemptStatusAssessed AttemptStatus = "assessed"
	AttemptStatusRetrieve AttemptStatus = "retrieve"
)

type MakeAttempt struct {
	git.RepoID
	// FileInfo Content make zip
}

type Attempt struct {
	ID        uuid.UUID `json:"id"        binding:"required"`
	CreatedAt time.Time `json:"createdAt" binding:"required"`
	Name      string    `json:"name"      binding:"required"`
}
