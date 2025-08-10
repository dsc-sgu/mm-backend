package attempt

import (
	"time"

	"github.com/google/uuid"
)

/*
Варианты работы с попытками:
- Студент отправил попытку с файлом через форму
- Студент отправил попытку с файлом через CLI
- Преподаватель решил оценить попытку
*/

// Add table with Users Git Data to DB
type AttemptStatus string

const (
	AttemptStatusSent     AttemptStatus = "sent"
	AttemptStatusAssessed AttemptStatus = "assessed"
	AttemptStatusRetrieve AttemptStatus = "retrieve"
)

// массив файлов
type FileInfo struct {
	FileName string `json:"fileName"    binding:"required"`
	// FilePath    string    `json:"-" binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
}

type RepoID struct {
	CourseId      uuid.UUID `json:"courseId"      binding:"required"`
	TaskId        uuid.UUID `json:"taskId"        binding:"required"`
	ParticipantID uuid.UUID `json:"participantId" binding:"required"`
}

type MakeAttempt struct {
	RepoID
	// FileInfo ... Content make zip
}

type Attempt struct {
	Id        uuid.UUID `json:"id"         binding:"required"`
	CreatedAt time.Time `json:"created_at" binding:"required"`
	Name      string    `json:"name"       binding:"required"`
}
