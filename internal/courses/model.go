package courses

import (
	"time"

	"github.com/google/uuid"
)

type Course struct {
	ID           uuid.UUID `json:"id"           db:"id"            binding:"required"`
	DisciplineID uuid.UUID `json:"disciplineID" db:"discipline_id"`
	OwnerID      uuid.UUID `json:"ownerID"      db:"owner_id"      binding:"required"`
	Name         string    `json:"name"         db:"name"          binding:"required"`
	DisplayName  string    `json:"displayName"  db:"display_name"  binding:"required"`

	CreatedAt time.Time `json:"createdAt" db:"created_at" binding:"required"`
}

type CreateCourse struct {
	DisciplineID uuid.UUID `json:"disciplineID" db:"discipline_id"`
	Name         string    `json:"name"         db:"name"          binding:"required"`
	DisplayName  string    `json:"displayName"  db:"display_name"  binding:"required"`
}

type UpdateCourse struct {
	OwnerID     uuid.UUID `json:"ownerID"     db:"owner_id"`
	DisplayName string    `json:"displayName" db:"display_name" binding:"required"`
}

type CoursePagination struct {
	Limit  int       `query:"limit"`
	LastID uuid.UUID `query:"last_id"`
}

type CourseIDResponse struct {
	ID uuid.UUID `json:"id"`
}

type CourseMemberRole string

const (
	StudentRole CourseMemberRole = "STUDENT"
	TeacherRole CourseMemberRole = "TEACHER"
)

type CourseMember struct {
	UserID    uuid.UUID        `json:"userID"    db:"user_id"`
	CourseID  uuid.UUID        `json:"courseID"  db:"course_id"`
	Role      CourseMemberRole `json:"role"      db:"role"`
	InvitedBy uuid.NullUUID    `json:"invitedBy" db:"invited_by"`
	IsActive  bool             `json:"isActive"  db:"is_active"`
}

type Student struct {
	UserID        uuid.UUID `json:"userID"        db:"user_id"`
	CourseID      uuid.UUID `json:"courseID"      db:"course_id"`
	AdmissionDate time.Time `json:"admissionDate" db:"admission_date"`
	IsActive      bool      `json:"isActive"      db:"is_active"`
}

type Teacher struct {
	UserID     uuid.UUID `json:"userID"     db:"user_id"`
	CourseID   uuid.UUID `json:"courseID"   db:"course_id"`
	PromotedBy uuid.UUID `json:"promotedBy" db:"promoted_by"`
	PromotedAt time.Time `json:"promotedAt" db:"promoted_at"`
	IsActive   bool      `json:"isActive"   db:"is_active"`
}

type Invite struct {
	ID           uuid.UUID        `json:"id"           db:"id"`
	CourseID     uuid.UUID        `json:"courseID"     db:"course_id"`
	ProvidedRole CourseMemberRole `json:"providedRole" db:"provided_role"`
	CreatedBy    uuid.UUID        `json:"createdBy"    db:"created_by"`
	CreatedAt    time.Time        `json:"createdAt"    db:"created_at"`
	ExpiresAt    time.Time        `json:"expiresAt"    db:"expires_at"`
	IsRevoked    bool             `json:"isRevoked"    db:"is_revoked"`
}

type CreateInvite struct {
	CourseID     uuid.UUID        `json:"courseID"     binding:"required"`
	ProvidedRole CourseMemberRole `json:"providedRole" binding:"required"`
	ExpiresAt    time.Time        `json:"expiresAt"    binding:"required"`
}

type InviteDetails struct {
	ID           uuid.UUID        `json:"id"`
	CourseID     uuid.UUID        `json:"courseID"`
	CourseName   string           `json:"courseName"`
	ProvidedRole CourseMemberRole `json:"providedRole"`
	CreatedBy    uuid.UUID        `json:"createdBy"`
	CreatedAt    time.Time        `json:"createdAt"`
	ExpiresAt    time.Time        `json:"expiresAt"`
	IsRevoked    bool             `json:"isRevoked"`
}

type UserRoleResponse struct {
	Role CourseMemberRole `json:"role"`
}
