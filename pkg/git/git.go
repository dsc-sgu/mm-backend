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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	Push(string, ssh.PublicKey, []string)
	Fetch(string, ssh.PublicKey)
}

// Middleware adds Git server functionality to the ssh.Server. Repos are stored
// in the specified repo directory. The provided Hooks implementation will be
// checked for access on a per repo basis for a ssh.Session public key.
// Hooks.Push and Hooks.Fetch will be called on successful completion of
// their commands.
func Middleware(
	repoDir string,
	repoRename func(string, gossh.PublicKey) (string, error),
	gh Hooks,
) wish.Middleware {
	return func(sh ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			cmd := s.Command()
			if len(cmd) == 2 {
				gc := cmd[0]
				pk := s.PublicKey()
				repo, err := repoRename(cmd[1], pk)
				if err != nil {
					Fatal(s, err)
				}
				access := gh.AuthRepo(repo, pk)
				switch gc {
				case "git-receive-pack":
					switch access {
					case ReadWriteAccess, AdminAccess:
						pushOptions, err := gitPack(s, gc, repoDir, repo)
						if err != nil {
							Fatal(s, ErrSystemMalfunction)
						} else {
							gh.Push(cmd[1], pk, pushOptions)
						}
					default:
						Fatal(s, ErrNotAuthed)
					}
					return
				case "git-upload-archive", "git-upload-pack":
					switch access {
					case ReadOnlyAccess, ReadWriteAccess, AdminAccess:
						_, err := gitPack(s, gc, repoDir, repo)
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

func gitPack(s ssh.Session, gitCmd string, repoDir string, repo string) ([]string, error) {
	cmd := strings.TrimPrefix(gitCmd, "git-")
	rp := filepath.Join(repoDir, repo)
	switch gitCmd {
	case "git-upload-archive", "git-upload-pack":
		exists, err := fileExists(rp)
		if !exists {
			return nil, ErrInvalidRepo
		}
		if err != nil {
			return nil, err
		}
		return nil, runGit(s, "", cmd, rp)
	case "git-receive-pack":
		err := EnsureRepo(repoDir, repo)
		if err != nil {
			return nil, err
		}

		pr, pw := io.Pipe()

		usi := exec.CommandContext(s.Context(), "git", "-c", "receive.advertisePushOptions=true", cmd, rp)
		usi.Stdout = s
		usi.Stdin = pr

		if err := usi.Start(); err != nil {
			return nil, err
		}

		var pushOptions []string
		optsCh := make(chan []string, 1)
		go func() {
			opts := extractPushOptions(s, pw)
			optsCh <- opts
		}()

		runErr := usi.Wait()
		pushOptions = <-optsCh

		if runErr != nil {
			return nil, runErr
		}

		err = ensureDefaultBranch(s, rp)
		if err != nil {
			return nil, err
		}

		return pushOptions, runGit(s, rp, "update-server-info")
	default:
		return nil, fmt.Errorf("unknown git command: %s", gitCmd)
	}
}

func fileExists(path string) (bool, error) {
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
	exists, err := fileExists(dir)
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
	exists, err = fileExists(rp)
	if err != nil {
		return err
	}
	if !exists {
		_, err := git.PlainInit(rp, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func runGit(s ssh.Session, dir string, args ...string) error {
	usi := exec.CommandContext(s.Context(), "git", args...)
	usi.Dir = dir
	usi.Stdout = s
	usi.Stdin = s
	if err := usi.Run(); err != nil {
		return err
	}
	return nil
}

// readPktLine reads one pkt-line from git protocol.
// Returns (data, isFlush, error).
func readPktLine(r io.Reader) ([]byte, bool, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, false, err
	}
	if string(header) == "0000" {
		return nil, true, nil
	}
	length, err := strconv.ParseUint(string(header), 16, 16)
	if err != nil {
		return nil, false, fmt.Errorf("parse pkt-len: %w", err)
	}
	if length < 4 {
		return nil, false, fmt.Errorf("invalid pkt-len: %d", length)
	}
	data := make([]byte, length-4)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, false, err
	}
	return data, false, nil
}

// extractPushOptions reads from r, extracts push options from git-receive-pack
// pkt-lines, and forwards all original data (including what was consumed for
// parsing) to w. Returns the extracted options. w is closed when all data
// has been forwarded.
//
// Must be called after git receive-pack has started (so the client has already
// received the reference advertisement and begun sending data).
func extractPushOptions(r io.Reader, w io.WriteCloser) []string {
	buf := &bytes.Buffer{}
	tr := io.TeeReader(r, buf)

	var options []string

	// Read ref-update pkt-lines until flush
	for {
		_, flush, err := readPktLine(tr)
		if err != nil {
			break
		}
		if flush {
			break
		}
	}

	// Peek at next 4 bytes to detect push options vs packfile
	peek := make([]byte, 4)
	if _, err := io.ReadFull(tr, peek); err != nil {
		goto forward
	}

	if string(peek) != "PACK" {
		for {
			hexLen := string(peek)
			if hexLen == "0000" {
				break
			}
			length, err := strconv.ParseUint(hexLen, 16, 16)
			if err != nil {
				break
			}
			data := make([]byte, length-4)
			if _, err := io.ReadFull(tr, data); err != nil {
				break
			}
			options = append(options, string(data))
			if _, err := io.ReadFull(tr, peek); err != nil {
				break
			}
		}
	}

forward:
	go func() {
		io.Copy(w, io.MultiReader(buf, r))
		w.Close()
	}()

	return options
}

func ensureDefaultBranch(s ssh.Session, repoPath string) error {
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
		err = runGit(s, repoPath, "branch", "-M", fb.Name().Short())
		if err != nil {
			return err
		}
	}
	if err != nil && err != plumbing.ErrReferenceNotFound {
		return err
	}
	return nil
}
