package gitservice

// An example git server. This will list all available repos if you ssh
// directly to the server. To test `ssh -p 23233 localhost` once it's running.

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"os"
	"strings"

	"github.com/dsc-sgu/mm-backend/pkg/git"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	gogit "github.com/go-git/go-git/v6"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"
)

const (
	port    = "2222"
	host    = "localhost"
	repoDir = "repos"
)

type App struct {
	Access git.AccessLevel
}

func (a App) AuthRepo(string, ssh.PublicKey) git.AccessLevel {
	return a.Access
}

func (a App) Push(repo string, _ ssh.PublicKey) {
	log.Info("push", "repo", repo)
}

func (a App) Fetch(repo string, _ ssh.PublicKey) {
	log.Info("fetch", "repo", repo)
}

func RepoRename(original string, publicKey gossh.PublicKey) (string, error) {
	fingerprint := gossh.FingerprintSHA256(publicKey)
	repoID, err := GetRepoId(original, fingerprint)
	if err != nil {
		return "", err
	}

	println(repoID.intoPath())

	repoPath := fmt.Sprintf("%s.git", repoID.intoPath())

	return repoPath, nil
}

type RepoID struct {
	CourseId      uuid.UUID `json:"courseId"      binding:"required"`
	TaskId        uuid.UUID `json:"taskId"        binding:"required"`
	ParticipantID uuid.UUID `json:"participantId" binding:"required"`
}

func (repoID *RepoID) intoPath() string {
	hasher := sha1.New()
	// NOTE: error shouldn't happen
	data, _ := json.Marshal(repoID)

	hasher.Write(data)
	hashSum := hasher.Sum(nil)
	return hex.EncodeToString(hashSum)
}

func GetParticipant(fingerprint string) (uuid.UUID, error) {
	if fingerprint == "SHA256:AH71wflD7hbxs0bGhssvTy77dLoszYUWXkeK798ph04" {
		return uuid.Parse("681ae49f-1f56-4632-bd1b-7ca3ab09a467")
	} else {
		return uuid.Parse("e7cf6012-1348-434b-9d54-bd89c9e6e95e")
	}
}

func GetCourse(name string) (uuid.UUID, error) {
	return uuid.Parse("e7cf6012-1348-434b-9d54-bd89c9e6e95e")
}

func GetTask(name string) (uuid.UUID, error) {
	return uuid.Parse("e7cf6012-1348-434b-9d54-bd89c9e6e95e")
}

func GetRepoId(path string, fingerprint string) (RepoID, error) {
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

	// TODO: this garbage of a language doesn't have optionals, so I don't know what could go
	// wrong because these are unset. Probably nothing, since all the errors are returned.
	var courseID uuid.UUID
	var taskID uuid.UUID
	if len(pathList) == 1 {
		// TODO: right now, i don't know what will be the interface for getting a task that
		// is course-wide.
		return RepoID{}, fmt.Errorf("course-wide tasks are not implemented")
	} else if len(pathList) == 2 {
		var err error
		courseID, err = GetCourse(pathList[0])
		if err != nil {
			return RepoID{}, err
		}
		taskID, err = GetTask(pathList[0])
		if err != nil {
			return RepoID{}, err
		}
	}

	participantID, err := GetParticipant(fingerprint)
	if err != nil {
		return RepoID{}, err
	}

	return RepoID{ParticipantID: participantID, CourseId: courseID, TaskId: taskID}, nil
}

type RepoManager interface {
	InitRepo(repoID RepoID) error
	// RemoveRepo(repoID RepoID) error
	// MakeAttempt(repoID RepoID, fileInfo []FileInfo) (attempt Attempt, err error)
	// GetAttempts(repoID RepoID) ([]Attempt, error)
	// GetDiff(attemptID1, attemptID2 uuid.UUID) ([]string, error)
}

type GitManager struct{}

var _ RepoManager = &GitManager{}

func (mng *GitManager) InitRepo(repoID RepoID) error {
	repoName := repoID.intoPath()

	repoPath := fmt.Sprintf("%s/%s.git", repoDir, repoName)
	repo, err := gogit.PlainInit(repoPath, true)
	if err != nil {
		return err
	}
	fmt.Println(repo)

	return nil
}

func CheckPubkeyAuth(ctx ssh.Context, pk ssh.PublicKey) bool {
	return true
}

func CheckPasswordAuth(ctx ssh.Context, password string) bool {
	return false
}

// Normally we would use a Bubble Tea program for the TUI but for simplicity,
// we'll just write a list of the pushed repos to the terminal and exit the ssh
// session.
func GitListMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		// Git will have a command included so only run this if there are no
		// commands passed to ssh.
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
			wish.Println(sess, fmt.Sprintf("git clone ssh://%s/%s", net.JoinHostPort(host, port), dir.Name()))
		}
		wish.Printf(sess, "\n\n### Add some repos! ###\n\n")
		wish.Printf(sess, "> cd some_repo\n")
		wish.Printf(sess, "> git remote add wish_test ssh://%s/some_repo\n", net.JoinHostPort(host, port))
		wish.Printf(sess, "> git push wish_test\n\n\n")
		next(sess)
	}
}
