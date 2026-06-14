package git

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	gogit "github.com/go-git/go-git/v6"
	"github.com/google/uuid"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	"github.com/dsc-sgu/mm-backend/pkg/git"
)

const (
	port    = "2222"
	host    = "localhost"
	repoDir = "repos"
)

type Service struct {
	repo_db DBRepo
}

func NewService(repo_db DBRepo) *Service {
	return &Service{repo_db: repo_db}
}

func (s *Service) RepoRename(original string, publicKey gossh.PublicKey) (string, error) {
	fingerprint := gossh.FingerprintSHA256(publicKey)
	repoID, err := s.GetRepoID(original, fingerprint)
	if err != nil {
		return "", err
	}

	println(repoID.IntoPath())

	repoPath := fmt.Sprintf("%s.git", repoID.IntoPath())

	return repoPath, nil
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
	return s.repo_db.AddSshKey(&key)
}

func (s *Service) DeleteSshKey(sessionId uuid.UUID, model *DeleteSshKey) error {
	return s.repo_db.DeleteSshKey(sessionId, model.Fingerprint)
}

func (s *Service) GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error) {
	return []string{"diff placeholder"}, nil
}

func (s *Service) InitRepo(repoID RepoID) error {
	repoName := repoID.IntoPath()
	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)
	_, err := gogit.PlainInit(repoPath, true)
	if err != nil {
		return fmt.Errorf("init repo: %w", err)
	}
	zap.L().Info("repository initialized", zap.String("path", repoPath))
	return nil
}

func (s *Service) RemoveRepo(repoID RepoID) error {
	repoName := repoID.IntoPath()
	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)
	err := os.RemoveAll(repoPath)
	if err != nil {
		return fmt.Errorf("remove repo: %w", err)
	}
	zap.L().Info("repository removed", zap.String("path", repoPath))
	return nil
}

func (s *Service) CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool {
	return s.repo_db.CheckPubkeyAuth(ctx, pk)
}

func (s *Service) CheckPasswordAuth(ctx ssh.Context, password string) bool {
	return s.repo_db.CheckPasswordAuth(ctx, password)
}

func (s *Service) AuthRepo(repo string, pk ssh.PublicKey) git.AccessLevel {
	if s.repo_db.CheckPubkeyAuth(nil, pk) {
		return git.ReadWriteAccess
	}
	return git.NoAccess
}

func (s *Service) Push(repo string, pk ssh.PublicKey, options []string) {
	zap.L().Info("Push hook called", zap.String("repo", repo), zap.Strings("options", options))

	if !hasAttemptConfirm(options) {
		return
	}

	fingerprint := gossh.FingerprintSHA256(pk)
	repoID, err := s.GetRepoID(repo, fingerprint)
	if err != nil {
		zap.L().Error("Push: get repoID", zap.Error(err))
		return
	}

	repoPath := filepath.Join(repoDir, repoID.IntoPath()+".git")
	bareRepo, err := gogit.PlainOpen(repoPath)
	if err != nil {
		zap.L().Error("Push: open repo", zap.Error(err))
		return
	}

	head, err := bareRepo.Head()
	if err != nil {
		zap.L().Error("Push: get HEAD", zap.Error(err))
		return
	}

	if err := s.repo_db.SaveAttempt(repoID, head.Hash().String()); err != nil {
		zap.L().Error("Push: save attempt", zap.Error(err))
	}
}

func hasAttemptConfirm(options []string) bool {
	for _, opt := range options {
		if opt == "attempt=confirm" {
			return true
		}
	}
	return false
}

func (s *Service) Fetch(repo string, pk ssh.PublicKey) {
	zap.L().Info("Fetch hook called", zap.String("repo", repo))
}

func (s *Service) GetRepoID(path string, fingerprint string) (RepoID, error) {
	if strings.HasPrefix(path, string(os.PathSeparator)) {
		path = path[len(string(os.PathSeparator)):]
	}

	pathList := strings.Split(path, string(os.PathSeparator))
	l := len(pathList)
	if l == 0 {
		return RepoID{}, fmt.Errorf("path is empty")
	}

	last := pathList[l-1]
	suffix := ".git"
	if strings.HasSuffix(last, suffix) {
		pathList[l-1] = last[:len(last)-len(suffix)]
	}

	fmt.Println(pathList, len(pathList))

	var courseID uuid.UUID
	var taskID uuid.UUID
	if len(pathList) == 1 {
		return RepoID{}, fmt.Errorf("course-wide tasks are not implemented")
	} else if len(pathList) == 2 {
		var err error
		courseID, err = s.repo_db.GetCourse(pathList[0])
		if err != nil {
			return RepoID{}, err
		}
		taskID, err = s.repo_db.GetTask(pathList[1])
		if err != nil {
			return RepoID{}, err
		}
	}

	participantID, err := s.repo_db.GetParticipant(fingerprint)
	if err != nil {
		return RepoID{}, err
	}

	return RepoID{
		ParticipantID: participantID,
		CourseID:      courseID,
		TaskID:        taskID,
	}, nil
}

func (s *Service) GitListMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		if len(sess.Command()) != 0 {
			next(sess)
			return
		}

		dest, err := os.ReadDir(repoDir)
		if err != nil && err != fs.ErrNotExist {
			log.Error("Invalid repository", "error", err)
		}
		if len(dest) > 0 {
			_, _ = fmt.Fprintf(sess, "\n### Repo Menu ###\n\n")
		}
		for _, dir := range dest {
			wish.Println(sess, fmt.Sprintf("• %s - ", dir.Name()))
			wish.Println(
				sess,
				fmt.Sprintf(
					"git clone ssh://%s/%s",
					net.JoinHostPort(host, port),
					dir.Name(),
				),
			)
		}
		wish.Printf(sess, "\n\n### Add some repos! ###\n\n")
		wish.Printf(sess, "> cd some_repo\n")
		wish.Printf(
			sess,
			"> git remote add wish_test ssh://%s/some_repo\n",
			net.JoinHostPort(host, port),
		)
		wish.Printf(sess, "> git push wish_test\n\n\n")
		next(sess)
	}
}
