package attempt

import (
	"encoding/json"
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

// FileInfo массив файлов
type FileInfo struct {
	FileName string `json:"fileName" binding:"required"`
	// FilePath    string    `json:"-"           binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
}

type RepoID struct {
	CourseID      uuid.UUID `json:"courseID"      binding:"required"`
	TaskID        uuid.UUID `json:"taskID"        binding:"required"`
	ParticipantID uuid.UUID `json:"participantID" binding:"required"`
}

type MakeAttempt struct {
	git.RepoID
	// FileInfo Content make zip
}

// type Attempt struct {
// 	Id        uuid.UUID `json:"id"         binding:"required"`
// 	CreatedAt time.Time `json:"created_at" binding:"required"`
// 	Name      string    `json:"name"       binding:"required"`
// }

type Attempt struct {
	Id             uuid.UUID       `json:"id"                       db:"attempt_id"      binding:"required"`
	UserID         uuid.UUID       `json:"userId"                   db:"user_id"         binding:"required"`
	TaskID         uuid.UUID       `json:"taskId"                   db:"task_id"         binding:"required"`
	State          AttemptStatus   `json:"state"                    db:"state"           binding:"required"`
	TransitionAt   time.Time       `json:"transitionAt"             db:"transition_at"   binding:"required"`
	TransitionData json.RawMessage `json:"transitionData,omitempty" db:"transition_data"`
}
