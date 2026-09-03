package attempt

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AttemptStatus string

const (
	AttemptStatusSent     AttemptStatus = "sent"
	AttemptStatusAssessed AttemptStatus = "assessed"
	AttemptStatusRetrieve AttemptStatus = "retrieve"
)

type FileInfo struct {
	FileName    string    `json:"fileName"    binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
}

type AttemptCommitInfo struct {
	UserID      uuid.UUID
	TaskID      uuid.UUID
	CommitHash  string
	CourseID    uuid.UUID
	TaskGroupID uuid.UUID
}

type Attempt struct {
	Id             uuid.UUID       `json:"id"                       db:"attempt_id"      binding:"required"`
	UserID         uuid.UUID       `json:"userId"                   db:"user_id"         binding:"required"`
	TaskID         uuid.UUID       `json:"taskId"                   db:"task_id"         binding:"required"`
	State          AttemptStatus   `json:"state"                    db:"state"           binding:"required"`
	TransitionAt   time.Time       `json:"transitionAt"             db:"transition_at"   binding:"required"`
	TransitionData json.RawMessage `json:"transitionData,omitempty" db:"transition_data"`
}
