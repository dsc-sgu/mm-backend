package sshkeys

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type repoStub struct{ added *SSHKey }

func (r *repoStub) AddSSHKey(key *SSHKey) error              { r.added = key; return nil }
func (r *repoStub) DeleteSSHKey(uuid.UUID, string) error     { return nil }
func (r *repoStub) GetParticipant(string) (uuid.UUID, error) { return uuid.Nil, nil }

func TestAddRejectsMalformedPublicKey(t *testing.T) {
	repo := &repoStub{}
	err := NewService(repo).Add(uuid.New(), AddSSHKey{Name: "laptop", Key: "not-a-public-key"})
	require.EqualError(t, err, "malformed SSH public key")
	require.Nil(t, repo.added)
}
