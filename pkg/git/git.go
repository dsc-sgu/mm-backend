// MIT License
//
// Copyright (c) 2019-2023 Charmbracelet, Inc (Original Author)
// Copyright (c) 2025 Andrew Guschin (Modifications)
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package git provides custom SSH server which allows to inject reaction on push/fetch events.
// You can write implementation of `Hooks` interface and inject it into `Middleware` to get any custom logic you want.
package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gossh "golang.org/x/crypto/ssh"
)

// ErrNotAuthed represents unauthorized access.
var ErrNotAuthed = errors.New("you are not authorized to do this")

// ErrSystemMalfunction represents a general system error returned to clients.
var ErrSystemMalfunction = errors.New("something went wrong")

// ErrInvalidRepo represents an attempt to access a non-existent repo.
var ErrInvalidRepo = errors.New("invalid repo")

// AccessLevel is the level of access allowed to a repo.
type AccessLevel int

const (
	// NoAccess does not allow access to the repo.
	NoAccess AccessLevel = iota

	// ReadOnlyAccess allows read-only access to the repo.
	ReadOnlyAccess

	// ReadWriteAccess allows read and write access to the repo.
	ReadWriteAccess

	// AdminAccess allows read, write, and admin access to the repo.
	AdminAccess
)

// GitHooks is an interface that allows for custom authorization
// implementations and post push/fetch notifications. Prior to git access,
// AuthRepo will be called with the ssh.Session public key and the repo name.
// Implementers return the appropriate AccessLevel.
//
// Deprecated: use Hooks instead.
type GitHooks = Hooks // nolint: revive

// Hooks is an interface that allows for custom authorization
// implementations and post push/fetch notifications. Prior to git access,
// AuthRepo will be called with the ssh.Session public key and the repo name.
// Implementers return the appropriate AccessLevel.
type Hooks interface {
	AuthRepo(string, ssh.PublicKey) AccessLevel
	Push(string, ssh.PublicKey)
	Fetch(string, ssh.PublicKey)
}

// Middleware adds Git server functionality to the ssh.Server. Repos are stored
// in the specified repo directory. The provided Hooks implementation will be
// checked for access on a per repo basis for a ssh.Session public key.
// Hooks.Push and Hooks.Fetch will be called on successful completion of
// their commands.
func Middleware(
	repoDir string,
	RepoRename func(string, gossh.PublicKey) (string, error),
	gh Hooks,
) wish.Middleware {
	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			cmd := s.Command()
			if len(cmd) == 2 {
				gc := cmd[0]
				pk := s.PublicKey()
				repo, err := RepoRename(cmd[1], pk)
				if err != nil {
					Fatal(s, err)
					return
				}
				access := gh.AuthRepo(repo, pk)
				switch gc {
				case "git-receive-pack":
					switch access {
					case ReadWriteAccess, AdminAccess:
						err := GitPack(s, gc, repoDir, repo)
						if err != nil {
							Fatal(s, ErrSystemMalfunction)
						} else {
							gh.Push(cmd[1], pk)
						}
					default:
						Fatal(s, ErrNotAuthed)
					}
					return
				case "git-upload-archive", "git-upload-pack":
					switch access {
					case ReadOnlyAccess, ReadWriteAccess, AdminAccess:
						err := GitPack(s, gc, repoDir, repo)
						switch err {
						case ErrInvalidRepo:
							Fatal(s, ErrInvalidRepo)
						case nil:
							gh.Fetch(repo, pk)
						default:
							log.Error("unknown git error", "error", err)
							Fatal(s, ErrSystemMalfunction)
						}
					default:
						Fatal(s, ErrNotAuthed)
					}
					return
				}
			}
			sh(s)
		}
	}
}

func GitPack(s ssh.Session, gitCmd string, repoDir string, repo string) error {
	cmd := strings.TrimPrefix(gitCmd, "git-")
	rp := filepath.Join(repoDir, repo)
	switch gitCmd {
	case "git-upload-archive", "git-upload-pack":
		exists, err := FileExists(rp)
		if !exists {
			return ErrInvalidRepo
		}
		if err != nil {
			return err
		}
		return RunGit(s, "", cmd, rp)
	case "git-receive-pack":
		err := EnsureRepo(repoDir, repo)
		if err != nil {
			return err
		}
		err = RunGit(s, "", "-c", "receive.advertisePushOptions=true", "receive-pack", rp)
		if err != nil {
			return err
		}
		err = EnsureDefaultBranch(s, rp)
		if err != nil {
			return err
		}
		// Needed for git dumb http server
		return RunGit(s, rp, "update-server-info")
	default:
		return fmt.Errorf("unknown git command: %s", gitCmd)
	}
}

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// Fatal prints to the session's STDOUT as a git response and exit 1.
func Fatal(s ssh.Session, v ...interface{}) {
	msg := fmt.Sprint(v...)
	// hex length includes 4 byte length prefix and ending newline
	pktLine := fmt.Sprintf("%04x%s\n", len(msg)+5, msg)
	_, _ = wish.WriteString(s, pktLine)
	s.Exit(1) // nolint: errcheck
}

// EnsureRepo makes sure the given repo exists within the given dir, and that
// it is git repository.
//
// If path does not exist, it'll be created.
// If the path is not a git repo, it will be git init-ed as a bare repository.
func EnsureRepo(dir, repo string) error {
	exists, err := FileExists(dir)
	if err != nil {
		return err
	}
	if !exists {
		err = os.MkdirAll(dir, os.ModeDir|os.FileMode(0o700))
		if err != nil {
			return err
		}
	}
	rp := filepath.Join(dir, repo)
	exists, err = FileExists(rp)
	if err != nil {
		return err
	}
	if !exists {
		_, err := git.PlainInit(rp, true)
		if err != nil {
			return err
		}
	}
	if err := WritePostReceiveHook(rp); err != nil {
		return fmt.Errorf("write post-receive hook: %w", err)
	}
	if err := WritePreReceiveHook(rp); err != nil {
		return fmt.Errorf("write pre-receive hook: %w", err)
	}
	return nil
}

// WritePreReceiveHook writes a pre-receive hook that checks tag pushes for
// required file patterns defined in .patterns file.
// Only tag refs (refs/tags/*) are checked; branch refs are always allowed.
// The task position must be passed as a numeric push option (e.g. -o "1").
// Files are checked via git ls-tree on the tag's commit.
func WritePreReceiveHook(repoPath string) error {
	hooksDir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	script := `#!/bin/sh
PATTERNS_FILE="$GIT_DIR/.mm-patterns"

TASK_POS=""
if [ -n "$GIT_PUSH_OPTION_COUNT" ] && [ "$GIT_PUSH_OPTION_COUNT" -gt 0 ]; then
    i=0
    while [ $i -lt "$GIT_PUSH_OPTION_COUNT" ]; do
        eval "opt=\$GIT_PUSH_OPTION_$i"
        case "$opt" in
            ''|*[!0-9]*) ;;
            *) TASK_POS="$opt" ;;
        esac
        i=$((i+1))
    done
fi

while read OLD NEW REF; do
    case "$REF" in
        refs/heads/*) continue ;;
        refs/tags/*)
            [ "$OLD" != "0000000000000000000000000000000000000000" ] && echo "ERROR: tag updates not allowed" >&2 && exit 1
            ;;
        *) continue ;;
    esac
    [ -z "$TASK_POS" ] && continue
    [ -f "$PATTERNS_FILE" ] || continue
    PATTERNS=$(grep "^$TASK_POS:" "$PATTERNS_FILE" | cut -d: -f2-)
    [ -z "$PATTERNS" ] && continue
    FILES=$(git ls-tree --name-only -r "$NEW" 2>/dev/null)
    [ -z "$FILES" ] && continue
    for file in $FILES; do
        for pattern in $PATTERNS; do
            case "$file" in
                $pattern) exit 0 ;;
            esac
        done
    done
    echo "ERROR: no files match required patterns for task $TASK_POS ($PATTERNS)" >&2
    exit 1
done
`
	return os.WriteFile(
		filepath.Join(hooksDir, "pre-receive"),
		[]byte(script), 0o755,
	)
}

// WritePostReceiveHook writes a post-receive hook that saves new tag commit hashes
// and push options to files for the Push hook to read.
func WritePostReceiveHook(repoPath string) error {
	hooksDir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	script := `#!/bin/sh
while read OLD NEW REF; do
  case "$REF" in refs/tags/*)
    [ "$OLD" = "0000000000000000000000000000000000000000" ] && echo "$NEW"
  ;; esac
done > "$GIT_DIR/push-tags"
if [ -n "$GIT_PUSH_OPTION_COUNT" ] && [ "$GIT_PUSH_OPTION_COUNT" -gt 0 ]; then
  i=0
  while [ $i -lt $GIT_PUSH_OPTION_COUNT ]; do
    eval "opt=\$GIT_PUSH_OPTION_$i"
    echo "$opt"
    i=$((i+1))
  done > "$GIT_DIR/push-options"
fi
`
	return os.WriteFile(
		filepath.Join(hooksDir, "post-receive"),
		[]byte(script), 0o755,
	)
}

func RunGit(s ssh.Session, dir string, args ...string) error {
	cmd := exec.CommandContext(s.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Stdout = s
	cmd.Stdin = s
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func EnsureDefaultBranch(s ssh.Session, repoPath string) error {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return err
	}
	brs, err := r.Branches()
	if err != nil {
		return err
	}
	defer brs.Close()
	fb, err := brs.Next()
	if err != nil {
		return err
	}
	// Rename the default branch to the first branch available
	_, err = r.Head()
	if err == plumbing.ErrReferenceNotFound {
		err = RunGit(s, repoPath, "branch", "-M", fb.Name().Short())
		if err != nil {
			return err
		}
	}
	if err != nil && err != plumbing.ErrReferenceNotFound {
		return err
	}
	return nil
}
