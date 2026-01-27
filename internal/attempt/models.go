package attempt

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
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
	FileName    string    `json:"fileName"    binding:"required"`
	FilePath    string    `json:"filePath"    binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
}

type MakeAttempt struct {
	RepoID
	// FileInfo Content make zip
}

type Attempt struct {
	ID        uuid.UUID `json:"id"        binding:"required"`
	CreatedAt time.Time `json:"createdAt" binding:"required"`
	Name      string    `json:"name"      binding:"required"`
}

type RepoID struct {
	CourseID      uuid.UUID `json:"courseID"      binding:"required"`
	TaskID        uuid.UUID `json:"taskID"        binding:"required"`
	ParticipantID uuid.UUID `json:"participantID" binding:"required"`
}

func (repoID *RepoID) IntoPath() string {
	hasher := sha1.New()
	// NOTE: error shouldn't happen
	data, _ := json.Marshal(repoID)

	hasher.Write(data)
	hashSum := hasher.Sum(nil)
	return hex.EncodeToString(hashSum)
}
