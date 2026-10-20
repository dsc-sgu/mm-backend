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

// Package git provides low-level primitives for serving Git repositories
// over SSH (running git-upload-pack/git-receive-pack, access-level types,
// pre/post-receive hooks). Callers that need auth, push/fetch notifications,
// or repo path resolution compose these primitives themselves.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/utils/ioutil"
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

type GitCmd string

const (
	GitUploadPack  GitCmd = "git-upload-pack"
	GitReceivePack GitCmd = "git-receive-pack"
)

const defaultWorkDir = ""

// GitPack serves a git-upload-pack or git-receive-pack session for repo.
// For receive-pack, env is passed to git and its hooks (see HookEnv).
func GitPack(s ssh.Session, gitCmd string, repoDir string, repo string, env []string) error {
	rp := filepath.Join(repoDir, repo)
	switch GitCmd(gitCmd) {
	case GitUploadPack:
		exists, err := FileExists(rp)
		if !exists {
			return ErrInvalidRepo
		}
		if err != nil {
			return err
		}
		gitRepo, err := gogit.PlainOpen(rp)
		if err != nil {
			return err
		}
		// UploadPack closes both streams once the pack is sent. Closing the
		// session there would drop it before its exit status is sent, so the
		// client would fail every non-empty clone or fetch.
		return transport.UploadPack(
			s.Context(), gitRepo.Storer, io.NopCloser(s), ioutil.WriteNopCloser(s),
			&transport.UploadPackRequest{},
		)
	case GitReceivePack:
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find server executable: %w", err)
		}
		if err := WriteHooks(rp, exe); err != nil {
			return err
		}
		if err := RunGit(s, defaultWorkDir, env,
			"-c", "receive.advertisePushOptions=true",
			"-c", "receive.denyNonFastForwards=true",
			"receive-pack", rp,
		); err != nil {
			return err
		}
		// receive-pack has already accepted the push, so a failure here must
		// not be reported to the caller as a rejected push.
		if err := EnsureDefaultBranch(s, rp); err != nil {
			log.Error("ensure default branch", "repo", rp, "error", err)
		}
		return nil
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

// RunGit runs git in dir with the session wired up as stdin/stdout (the pack
// protocol) and stderr mirrored both to the session's dedicated SSH stderr
// channel and to a buffer that gets logged on failure. Without this, stderr
// (unlike stdout) has nowhere to go: Go connects an unset cmd.Stderr to
// /dev/null, so anything git itself writes there - as opposed to what a
// pre/post-receive hook writes to its own stderr, which git relays to the
// client over the pack protocol's side-band regardless - was previously
// silently lost, on both the client and the server.
func RunGit(s ssh.Session, dir string, env []string, args ...string) error {
	cmd := exec.CommandContext(s.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = s
	cmd.Stdin = s
	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(s.Stderr(), &stderr)
	if err := cmd.Run(); err != nil {
		log.Error("git command failed", "args", args, "stderr", stderr.String(), "error", err)
		return err
	}
	return nil
}

func EnsureDefaultBranch(s ssh.Session, repoPath string) error {
	r, err := gogit.PlainOpen(repoPath)
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
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	// Rename the default branch to the first branch available
	_, err = r.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		err = RunGit(s, repoPath, nil, "branch", "-M", fb.Name().Short())
		if err != nil {
			return err
		}
	}
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}
	return nil
}
