package git

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type SSHKey struct {
	OwnerID     uuid.UUID `json:"ownerID"     db:"owner_id"    binding:"required"`
	Name        string    `json:"name"        db:"name"        binding:"required"`
	Key         string    `json:"key"         db:"key"         binding:"required"`
	Fingerprint string    `json:"fingerprint" db:"fingerprint" binding:"required"`
	CreatedAt   time.Time `json:"createdAt"   db:"created_at"  binding:"required"`
}

type AddSSHKey struct {
	Name string `json:"name" db:"name" binding:"required"`
	Key  string `json:"key"  db:"key"  binding:"required"`
}

type DeleteSSHKey struct {
	Fingerprint string `json:"fingerprint" db:"fingerprint" binding:"required"`
}

type RepoID struct {
	CourseID      uuid.UUID `json:"courseID"      binding:"required"`
	TaskGroupID   uuid.UUID `json:"taskGroupID"   binding:"required"`
	ParticipantID uuid.UUID `json:"participantID" binding:"required"`
}

func (repoID *RepoID) IntoPath() string {
	hasher := sha1.New()
	data, _ := json.Marshal(repoID)
	hasher.Write(data)
	hashSum := hasher.Sum(nil)
	return hex.EncodeToString(hashSum)
}

func TemplatePath(taskGroupID uuid.UUID) string {
	hasher := sha1.New()
	_, _ = fmt.Fprint(hasher, "template:", taskGroupID.String())
	hashSum := hasher.Sum(nil)
	return hex.EncodeToString(hashSum)
}

type FileInfo struct {
	FileName    string    `json:"fileName"    binding:"required"`
	FilePath    string    `json:"filePath"    binding:"required"`
	FileSize    int64     `json:"fileSize"    binding:"required"`
	ContentType string    `json:"contentType" binding:"required"`
	MD5Hash     string    `json:"md5Hash"     binding:"required"`
	UploadedAt  time.Time `json:"uploadedAt"  binding:"required"`
	Content     []byte    `json:"content"     binding:"required"`
}

type AttemptCommitInfo struct {
	UserID      uuid.UUID
	TaskID      uuid.UUID
	CommitHash  string
	CourseID    uuid.UUID
	TaskGroupID uuid.UUID
}
