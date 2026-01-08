package git

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
)

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}

func (s *Service) AddSshKey(sessionId uuid.UUID, model *AddSshKey) error {
	zap.L().Debug("Parsing key", zap.String("key", model.Key))
	authkey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(model.Key))
	if err != nil {
		return errors.New("malformed SSH public key")
	}
	pubkey, err := gossh.ParsePublicKey(authkey.Marshal())
	if err != nil {
		return errors.New("malformed SSH public key")
	}
	fingerprint := gossh.FingerprintSHA256(pubkey)
	key := SshKey{
		OwnerId:     sessionId,
		Name:        model.Name,
		Key:         string(gossh.MarshalAuthorizedKey(pubkey)),
		Fingerprint: fingerprint,
		CreatedAt:   time.Now(),
	}
	return s.repo.AddSshKey(&key)
}

func (s *Service) DeleteSshKey(sessionId uuid.UUID, model *DeleteSshKey) error {
	return s.repo.DeleteSshKey(sessionId, model.Fingerprint)
}
