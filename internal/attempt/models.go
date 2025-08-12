package attempt

import (
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

// Add table with Users Git Data to DB
type AttemptState string

const (
	AttemptStatusDraft    AttemptState = "draft"
	AttemptStatusSubmited AttemptState = "submitted"
	AttemptStatusGraded   AttemptState = "graded"
)

// массив файлов
type FileInfo struct {
	FileName string `json:"fileName" binding:"required"`
	// FilePath    string    `json:"-" binding:"required"`
	FileSize    int64     `json:"fileSize" binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash" binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt" binding:"required"`
	Content     []byte
}

type RepoID struct {
	CourseId      uuid.UUID `json:"courseId" binding:"required"`
	TaskId        uuid.UUID `json:"taskId" binding:"required"`
	ParticipantID uuid.UUID `json:"participantId" binding:"required"`
}

type MakeAttempt struct {
	RepoID `json:"repoId" binding:"required"`
	Files  []FileInfo `json:"files" binding:"required"`
}

// DB structs
type AttemptDB struct {
	Id       uuid.UUID `db:"id" binding:"required"`
	UserID   uuid.UUID `db:"user_id" binding:"required"`
	TaskID   uuid.UUID `db:"task_id" binding:"required"`
	CourseID uuid.UUID `db:"course_id" binding:"required"`
}

// DB structs
type AttemptTransitDB struct {
	Id             uuid.UUID       `json:"id" db:"id" binding:"required"`
	State          AttemptState    `json:"state" db:"state" binding:"required"`
	TransitionAt   time.Time       `json:"transitionAt" db:"transition_at" binding:"required"`
	TransitionData json.RawMessage `json:"transitionData" db:"transition_data" binding:"required"`
}

type Attempt struct {
	Id        uuid.UUID `json:"id" binding:"required"`
	CreatedAt time.Time `json:"created_at" binding:"required"`
	Name      string    `json:"name" binding:"required"`
}

type AttemptDetails struct {
	RepositoryId  uuid.UUID `json:"repositoryId" binding:"required"`
	RepositoryURL string    `json:"repositoryURL" binding:"required"`
	BranchName    string    `json:"branchName" binding:"required"`

	CreatedAt time.Time `json:"created_at" binding:"required"`
	UpdatedAt time.Time `json:"updated_at" binding:"required"`

	CommitsList []CommitDetail `json:"commitsList" binding:"required"`
}

// COMMENT(xseniva): i know it may be stupid =)
type AttemptResponse struct {
	AttemptTransitDB
	AttemptDetails
}

type AttemptUpdate struct {
	Id    uuid.UUID  `json:"id" binding:"required"`
	files []FileInfo `json:"files" binding:"required"`
}

type ReviewAttempt struct {
	Id uuid.UUID `json:"id" binding:"required"`
	RepoID
	AttemptId uuid.UUID `json:"attemptId" binding:"required"`

	Status     AttemptState `json:"status" binding:"required"`
	Grade      int          `json:"grade" binding:"required"`
	ReviewedAt time.Time    `json:"created_at" binding:"required"`
	Comment    string       `json:"created_at" binding:"required"`
}
