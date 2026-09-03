package sshkeys

import (
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"
)

type SSHKey struct {
	OwnerID     uuid.UUID `db:"owner_id"`
	Name        string    `json:"name" db:"name" binding:"required"`
	Key         string    `json:"key" db:"key" binding:"required"`
	Fingerprint string    `json:"fingerprint" db:"fingerprint"`
	CreatedAt   time.Time `db:"created_at"`
}

type AddSSHKey struct {
	Name string `json:"name" binding:"required"`
	Key  string `json:"key" binding:"required"`
}

type Service struct{ repo Repo }

func NewService(repo Repo) *Service { return &Service{repo: repo} }

func (s *Service) Add(ownerID uuid.UUID, model AddSSHKey) error {
	key, _, _, _, err := gossh.ParseAuthorizedKey([]byte(model.Key))
	if err != nil {
		return errors.New("malformed SSH public key")
	}
	return s.repo.AddSSHKey(&SSHKey{
		OwnerID:     ownerID,
		Name:        model.Name,
		Key:         strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))),
		Fingerprint: gossh.FingerprintSHA256(key),
		CreatedAt:   time.Now(),
	})
}
func (s *Service) Delete(ownerID uuid.UUID, fingerprint string) error {
	return s.repo.DeleteSSHKey(ownerID, fingerprint)
}
func (s *Service) ParticipantID(fingerprint string) (uuid.UUID, error) {
	return s.repo.GetParticipant(fingerprint)
}

// CheckPublicKeyAuth admits an SSH connection when its key is registered to a user.
func (s *Service) CheckPublicKeyAuth(_ ssh.Context, key ssh.PublicKey) bool {
	_, err := s.repo.GetParticipant(gossh.FingerprintSHA256(key))
	return err == nil
}

// CheckPasswordAuth denies password logins: Git access is key-only.
func (s *Service) CheckPasswordAuth(ssh.Context, string) bool { return false }
