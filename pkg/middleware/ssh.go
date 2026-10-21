package middleware

import (
	"errors"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"

	pkggit "github.com/dsc-sgu/mm-backend/pkg/git"
)

// GitRepositories stores the bare repositories git sessions run against.
type GitRepositories interface {
	Lock(pkggit.RepoID) (unlock func())
	EnsureRepo(pkggit.RepoID) error
	RepoPath(pkggit.RepoID) string
}

// SSHMiddleware serves Git repositories over SSH: it resolves the raw
// course/group SSH path to a RepoID, checks course membership, runs the
// git command, and records pushed attempts.
func SSHMiddleware(store pkggit.Store, repos GitRepositories) wish.Middleware {
	return func(sh ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			cmd := sess.Command()
			if len(cmd) != 2 {
				sh(sess)
				return
			}
			gitCmd := pkggit.GitCmd(cmd[0])
			if gitCmd != pkggit.GitUploadPack && gitCmd != pkggit.GitReceivePack {
				sh(sess)
				return
			}
			log := zap.L().With(zap.String("repo", cmd[1]))

			id, err := pkggit.ResolveRepo(sess.Context(), store, cmd[1], gossh.FingerprintSHA256(sess.PublicKey()))
			switch {
			case errors.Is(err, pkggit.ErrNotAuthed):
				log.Info("ssh repository access denied", zap.Error(err))
				pkggit.Fatal(sess, pkggit.ErrNotAuthed)
				return
			case errors.Is(err, pkggit.ErrInvalidRepo):
				log.Info("resolve ssh repository", zap.Error(err))
				pkggit.Fatal(sess, pkggit.ErrInvalidRepo)
				return
			case err != nil:
				log.Error("resolve ssh repository", zap.Error(err))
				pkggit.Fatal(sess, pkggit.ErrSystemMalfunction)
				return
			}

			unlock := repos.Lock(id)
			defer unlock()
			if err := repos.EnsureRepo(id); err != nil {
				log.Error("init ssh repository", zap.Error(err))
				pkggit.Fatal(sess, pkggit.ErrSystemMalfunction)
				return
			}
			err = pkggit.GitPack(sess, gitCmd, repos.RepoPath(id), id, store)
			switch {
			case err == nil:
			case errors.Is(err, pkggit.ErrPushRejected):
				// The client already got the reason in the push report.
				log.Info("push rejected", zap.Error(err))
			default:
				// The protocol may be half-written, so no pkt-line can follow.
				log.Error("git command failed", zap.Error(err))
				sess.Exit(1) // nolint: errcheck
			}
		}
	}
}
