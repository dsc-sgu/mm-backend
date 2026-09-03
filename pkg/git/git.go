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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
	GitUploadPack    GitCmd = "git-upload-pack"
	GitUploadArchive GitCmd = "git-upload-archive"
	GitReceivePack   GitCmd = "git-receive-pack"
)

const defaultWorkDir = ""

func GitPack(s ssh.Session, gitCmd string, repoDir string, repo string) error {
	cmd := strings.TrimPrefix(gitCmd, "git-")
	rp := filepath.Join(repoDir, repo)
	switch GitCmd(gitCmd) {
	case GitUploadArchive, GitUploadPack:
		exists, err := FileExists(rp)
		if !exists {
			return ErrInvalidRepo
		}
		if err != nil {
			return err
		}
		return RunGit(s, defaultWorkDir, cmd, rp)
	case GitReceivePack:
		if err := EnsureRepo(repoDir, repo); err != nil {
			return err
		}
		if err := RunGit(s, defaultWorkDir, "-c", "receive.advertisePushOptions=true", "receive-pack", rp); err != nil {
			return err
		}
		if err := EnsureDefaultBranch(s, rp); err != nil {
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
// required file patterns defined in the .mm-patterns file.
// Only tag refs (refs/tags/*) are checked; branch refs are always allowed.
// The submitted task is selected via the "submit=<name>" push option
// (e.g. -o submit=task1). Patterns are looked up by task name; the file is
// tab-separated ("<name>\t<glob>"). Files are checked via git ls-tree on the
// tag's commit.
func WritePreReceiveHook(repoPath string) error {
	hooksDir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	script := `#!/bin/sh
PATTERNS_FILE="$GIT_DIR/.mm-patterns"
TAB=$(printf '\t')

SUBMIT=""
if [ -n "$GIT_PUSH_OPTION_COUNT" ] && [ "$GIT_PUSH_OPTION_COUNT" -gt 0 ]; then
    i=0
    while [ $i -lt "$GIT_PUSH_OPTION_COUNT" ]; do
        eval "opt=\$GIT_PUSH_OPTION_$i"
        case "$opt" in
            submit=*) SUBMIT="${opt#submit=}" ;;
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
    [ -z "$SUBMIT" ] && continue
    [ -f "$PATTERNS_FILE" ] || continue
    PATTERNS=""
    while IFS="$TAB" read -r pname ppat; do
        [ "$pname" = "$SUBMIT" ] && PATTERNS="$PATTERNS $ppat"
    done < "$PATTERNS_FILE"
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
    echo "ERROR: no files match required patterns for task '$SUBMIT' ($PATTERNS)" >&2
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
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	// Rename the default branch to the first branch available
	_, err = r.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		err = RunGit(s, repoPath, "branch", "-M", fb.Name().Short())
		if err != nil {
			return err
		}
	}
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}
	return nil
}
