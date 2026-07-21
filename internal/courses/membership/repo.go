package membership

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	StudentRole Role = "STUDENT"
	TeacherRole Role = "TEACHER"
)

// Member is the database representation of a course member
type Member struct {
	UserID    uuid.UUID     `json:"userID"    db:"user_id"`
	CourseID  uuid.UUID     `json:"courseID"  db:"course_id"`
	Role      Role          `json:"role"      db:"role"`
	InvitedBy uuid.NullUUID `json:"invitedBy" db:"invited_by"`
	IsActive  bool          `json:"isActive"  db:"is_active"`
}

// Student is the database representation of a course student
type Student struct {
	UserID        uuid.UUID `json:"userID"        db:"user_id"`
	CourseID      uuid.UUID `json:"courseID"      db:"course_id"`
	AdmissionDate time.Time `json:"admissionDate" db:"admission_date"`
	IsActive      bool      `json:"isActive"      db:"is_active"`
}

// Teacher is the database representation of a course teacher
type Teacher struct {
	UserID     uuid.UUID `json:"userID"     db:"user_id"`
	CourseID   uuid.UUID `json:"courseID"   db:"course_id"`
	PromotedBy uuid.UUID `json:"promotedBy" db:"promoted_by"`
	PromotedAt time.Time `json:"promotedAt" db:"promoted_at"`
	IsActive   bool      `json:"isActive"   db:"is_active"`
}

// Invite is the database representation of a course invite
type Invite struct {
	ID           uuid.UUID  `json:"id"           db:"id"`
	CourseID     uuid.UUID  `json:"courseID"     db:"course_id"`
	ProvidedRole Role       `json:"providedRole" db:"provided_role"`
	CreatedBy    uuid.UUID  `json:"createdBy"    db:"created_by"`
	CreatedAt    time.Time  `json:"createdAt"    db:"created_at"`
	ExpiresAt    *time.Time `json:"expiresAt"    db:"expires_at"`
	IsRevoked    bool       `json:"isRevoked"    db:"is_revoked"`
}

// CreateInvite is the input for creating an invite,
// used by both the service and repository layers
type CreateInvite struct {
	CourseID     uuid.UUID  `json:"-"                   binding:"required"`
	ProvidedRole Role       `json:"providedRole"        binding:"required"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}

// InviteDetails is a course invite enriched with its course's name, fetched
// by the repository in a single joined query
type InviteDetails struct {
	ID           uuid.UUID  `json:"id"           db:"id"`
	CourseID     uuid.UUID  `json:"courseID"     db:"course_id"`
	CourseName   string     `json:"courseName"   db:"course_name"`
	ProvidedRole Role       `json:"providedRole" db:"provided_role"`
	CreatedBy    uuid.UUID  `json:"createdBy"    db:"created_by"`
	CreatedAt    time.Time  `json:"createdAt"    db:"created_at"`
	ExpiresAt    *time.Time `json:"expiresAt"    db:"expires_at"`
	IsRevoked    bool       `json:"isRevoked"    db:"is_revoked"`
}

type Repo interface {
	GetMember(ctx context.Context, userID, courseID uuid.UUID) (*Member, error)
	CreateInvite(
		ctx context.Context,
		model *CreateInvite,
		createdBy uuid.UUID,
	) (*Invite, error)
	GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*Invite, error)
	GetInviteDetailsByID(
		ctx context.Context,
		inviteID uuid.UUID,
	) (*InviteDetails, error)
	GetInvitesByCourseID(
		ctx context.Context,
		courseID uuid.UUID,
	) ([]Invite, error)
	EnrollUserByInvite(
		ctx context.Context,
		userID uuid.UUID,
		invite *Invite,
	) error
}
