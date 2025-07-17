package dto

import "github.com/google/uuid"

type AttemptType struct {
	Id        uuid.UUID `json:"id"`
	UserId    uuid.UUID `json:"userId"`
	TaskId    uuid.UUID `json:"taskId"`
	AttemptAt string    `json:"attemptAt"`
}
