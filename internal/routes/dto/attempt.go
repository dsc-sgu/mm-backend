package dto

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

type AttemptStatus string

const (
	AttemptStatusSent     AttemptStatus = "sent"
	AttemptStatusAssessed AttemptStatus = "assessed"
	AttemptStatusRetrieve AttemptStatus = "retrieve"
)

type FileInfo struct {
	FileName    string    `json:"fileName" binding:"required"`
	FilePath    string    `json:"-" binding:"required"`
	FileSize    int64     `json:"fileSize" binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash" binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt" binding:"required"`
}

type CreateAttempt struct {
	Id        uuid.UUID `json:"id" binding:"required"`
	UserId    uuid.UUID `json:"userId" binding:"required"`
	CourseId  uuid.UUID `json:"courseId" binding:"required"`
	TaskId    uuid.UUID `json:"taskId" binding:"required"`
	CreatedAt time.Time `json:"createdAt" binding:"required"`

	FileInfo

	RepositoryId uuid.UUID `json:"repositoryId" binding:"required"`
	BranchName   string    `json:"branchName" binding:"required"`
	CommitMess   string    `json:"commitId" binding:"required"`
}

type CommitDetail struct {
	CommitSHA string    `json:"commit_sha" binding:"required"`
	Message   string    `json:"commit_message" binding:"required"`
	Timestamp time.Time `json:"commit_timestamp" binding:"required"`

	AuthorName  string `json:"author_name" binding:"required"`
	AuthorEmail string `json:"author_email" binding:"required"`

	FilesAdded    []string `json:"files_added" binding:"required"`
	FilesModified []string `json:"files_modified" binding:"required"`
	FilesRemoved  []string `json:"files_removed" binding:"required"`
}

// From Backend to Frontend
type AssignmentAttempt struct {
	Id uuid.UUID `json:"id" binding:"required"`

	RepositoryId      uuid.UUID `json:"repositoryId" binding:"required"`
	BranchName        string    `json:"branchName" binding:"required"`
	CommitsCount      int       `json:"commitsCount" binding:"required"`
	WebhookDeliveryID string    `json:"webhook_delivery_id" binding:"required"`

	CreatedAt time.Time `json:"created_at" binding:"required"`
	UpdatedAt time.Time `json:"updated_at" binding:"required"`

	CommitsList []CommitDetail `json:"commitsList" binding:"required"`
}

type ReviewAttempt struct {
	Id        uuid.UUID `json:"id" binding:"required"`
	UserId    uuid.UUID `json:"userId" binding:"required"`
	CourseId  uuid.UUID `json:"courseId" binding:"required"`
	TaskId    uuid.UUID `json:"taskId" binding:"required"`
	AttemptId uuid.UUID `json:"attemptId" binding:"required"`

	Status     AttemptStatus `json:"status" binding:"required"`
	Grade      int           `json:"grade" binding:"required"`
	ReviewedAt time.Time     `json:"created_at" binding:"required"`
	Comment    string        `json:"created_at" binding:"required"`
}
