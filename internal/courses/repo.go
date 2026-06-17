package courses

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type CourseMemberRole string

const (
	StudentRole CourseMemberRole = "STUDENT"
	TeacherRole CourseMemberRole = "TEACHER"
)

// Course is the database representation of a course.
type Course struct {
	ID               uuid.UUID  `json:"id"                         db:"id"                 binding:"required"`
	DisciplineID     uuid.UUID  `json:"disciplineID,omitempty"     db:"discipline_id"`
	ActiveSnapshotID *uuid.UUID `json:"activeSnapshotID,omitempty" db:"active_snapshot_id"`
	OwnerID          uuid.UUID  `json:"ownerID"                    db:"owner_id"           binding:"required"`
	Name             string     `json:"name"                       db:"name"               binding:"required"`
	Version          int        `json:"version"                    db:"version"            binding:"required"`
	CreatedAt        time.Time  `json:"createdAt"                  db:"created_at"         binding:"required"`
	DeletedAt        *time.Time `json:"deletedAt,omitempty"        db:"deleted_at"`
}

// CreateCourse is the input for creating a course, used by both the service and repository layers.
type CreateCourse struct {
	DisciplineID uuid.UUID `json:"disciplineID,omitempty" db:"discipline_id"`
	Name         string    `json:"name"                   db:"name"          binding:"required"`
}

// UpdateCourse is the input for updating a course, used by both the service and repository layers.
type UpdateCourse struct {
	OwnerID uuid.UUID `json:"ownerID,omitempty" db:"owner_id"`
	Name    string    `json:"name,omitempty"    db:"name"`
}

type PublishSnapshot struct {
	CourseID        uuid.UUID `json:"courseID"`
	NewSnapshotID   uuid.UUID `json:"newSnapshotID"`
	ExpectedVersion int       `json:"expectedVersion"`
}

// CourseMember is the database representation of a course member.
type CourseMember struct {
	UserID    uuid.UUID        `json:"userID"    db:"user_id"`
	CourseID  uuid.UUID        `json:"courseID"  db:"course_id"`
	Role      CourseMemberRole `json:"role"      db:"role"`
	InvitedBy uuid.NullUUID    `json:"invitedBy" db:"invited_by"`
	IsActive  bool             `json:"isActive"  db:"is_active"`
}

// Student is the database representation of a course student.
type Student struct {
	UserID        uuid.UUID `json:"userID"        db:"user_id"`
	CourseID      uuid.UUID `json:"courseID"      db:"course_id"`
	AdmissionDate time.Time `json:"admissionDate" db:"admission_date"`
	IsActive      bool      `json:"isActive"      db:"is_active"`
}

// Teacher is the database representation of a course teacher.
type Teacher struct {
	UserID     uuid.UUID `json:"userID"     db:"user_id"`
	CourseID   uuid.UUID `json:"courseID"   db:"course_id"`
	PromotedBy uuid.UUID `json:"promotedBy" db:"promoted_by"`
	PromotedAt time.Time `json:"promotedAt" db:"promoted_at"`
	IsActive   bool      `json:"isActive"   db:"is_active"`
}

// Invite is the database representation of a course invite.
type Invite struct {
	ID           uuid.UUID        `json:"id"           db:"id"`
	CourseID     uuid.UUID        `json:"courseID"     db:"course_id"`
	ProvidedRole CourseMemberRole `json:"providedRole" db:"provided_role"`
	CreatedBy    uuid.UUID        `json:"createdBy"    db:"created_by"`
	CreatedAt    time.Time        `json:"createdAt"    db:"created_at"`
	ExpiresAt    time.Time        `json:"expiresAt"    db:"expires_at"`
	IsRevoked    bool             `json:"isRevoked"    db:"is_revoked"`
}

// CreateInvite is the input for creating an invite, used by both the service and repository layers.
type CreateInvite struct {
	CourseID     uuid.UUID        `json:"courseID"     binding:"required"`
	ProvidedRole CourseMemberRole `json:"providedRole" binding:"required"`
	ExpiresAt    time.Time        `json:"expiresAt"    binding:"required"`
}

// InviteDetails is built by the service layer and returned to the handler.
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

type Repo interface {
	CreateCourse(
		ctx context.Context,
		tx *sqlx.Tx,
		model *CreateCourse,
		ownerID uuid.UUID,
	) (*Course, error)
	GetCourseByID(ctx context.Context, id uuid.UUID) (*Course, error)
	GetPaginatedCourses(
		ctx context.Context,
		limit int,
		lastID uuid.UUID,
	) ([]Course, error)
	UpdateCourseByID(
		ctx context.Context,
		id uuid.UUID,
		update *UpdateCourse,
	) (*Course, error)
	DeleteCourseByID(ctx context.Context, id uuid.UUID) error
	PublishSnapshotToCourse(
		ctx context.Context,
		tx *sqlx.Tx,
		model *PublishSnapshot,
	) error
	CreateInvite(
		ctx context.Context,
		model *CreateInvite,
		createdBy uuid.UUID,
	) (*Invite, error)
	GetInviteByID(ctx context.Context, inviteID uuid.UUID) (*Invite, error)
	EnrollUserByInvite(
		ctx context.Context,
		userID uuid.UUID,
		invite *Invite,
	) error
	GetCourseMember(
		ctx context.Context,
		userID, courseID uuid.UUID,
	) (*CourseMember, error)
}
