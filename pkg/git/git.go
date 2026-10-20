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
// For receive-pack, patterns maps every live task of the repository's group
// to its file patterns; the pre-receive hook checks submissions against it.
func GitPack(s ssh.Session, gitCmd string, repoDir string, repo string, patterns map[string][]string) error {
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
		if err := WritePostReceiveHook(rp); err != nil {
			return fmt.Errorf("write post-receive hook: %w", err)
		}
		if err := WritePreReceiveHook(rp); err != nil {
			return fmt.Errorf("write pre-receive hook: %w", err)
		}
		patternsFile, err := writePatternsFile(patterns)
		if err != nil {
			return fmt.Errorf("write patterns file: %w", err)
		}
		defer func() {
			if err := os.Remove(patternsFile); err != nil {
				log.Error("remove patterns file", "error", err, "path", patternsFile)
			}
		}()
		if err := RunGit(s, defaultWorkDir, []string{"MM_PATTERNS_FILE=" + patternsFile},
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

// WritePreReceiveHook writes a pre-receive hook that rejects tag updates and
// checks a new tag pushed with "-o submit=<task>": the task must exist, and if
// it has patterns, the tagged commit must contain a file matching one of them.
// Branch pushes are not checked. The task list comes from the file named by
// MM_PATTERNS_FILE (see GitPack and formatPatterns).
func WritePreReceiveHook(repoPath string) error {
	hooksDir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	script := `#!/bin/sh
TAB=$(printf '\t')
ZERO=0000000000000000000000000000000000000000

# Prints the value of the last "submit=<task>" push option.
submitted_task() {
    i=0
    while [ "$i" -lt "${GIT_PUSH_OPTION_COUNT:-0}" ]; do
        eval "opt=\$GIT_PUSH_OPTION_$i"
        case "$opt" in
            submit=*) printf '%s\n' "${opt#submit=}" ;;
        esac
        i=$((i+1))
    done | tail -n 1
}

# Prints the patterns of task $1, one per line. Fails if there is no such task.
task_patterns() {
    found=1
    while IFS="$TAB" read -r name pattern; do
        if [ "$name" = "$1" ]; then
            found=0
            [ -n "$pattern" ] && printf '%s\n' "$pattern"
        fi
    done < "$MM_PATTERNS_FILE"
    return $found
}

# Succeeds if commit $1 has a file matching one of the patterns in $2.
has_matching_file() {
    while IFS= read -r file; do
        while IFS= read -r pattern; do
            case "$file" in
                $pattern) return 0 ;;
            esac
        done <<EOF
$2
EOF
    done <<EOF
$(git -c core.quotePath=false ls-tree -r --name-only "$1")
EOF
    return 1
}

task=$(submitted_task)
while read -r old new ref; do
    case "$ref" in
        refs/tags/*) ;;
        *) continue ;;
    esac
    if [ "$old" != "$ZERO" ]; then
        echo "ERROR: tag updates not allowed" >&2
        exit 1
    fi
    [ -z "$task" ] && continue
    if ! patterns=$(task_patterns "$task"); then
        echo "ERROR: no task named '$task' in this group" >&2
        exit 1
    fi
    [ -z "$patterns" ] && continue
    if ! has_matching_file "$new" "$patterns"; then
        echo "ERROR: no files match required patterns for task '$task'" >&2
        exit 1
    fi
done
`
	return os.WriteFile(
		filepath.Join(hooksDir, "pre-receive"),
		[]byte(script), 0o755,
	)
}

// WritePostReceiveHook writes a post-receive hook that saves new tag commit
// hashes and push options to files for onPush to read. Both files are
// unconditionally (re)written on every push, even when a file would end up
// empty: onPush deletes them once it has read them, but only after a
// successful push all the way through GitPack, so if a push option was
// written and something downstream of receive-pack (e.g. GitPack's
// post-processing) failed before onPush ran, the file would otherwise be
// left on disk and misread by a later, unrelated push that carries no
// options of its own.
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
{
  if [ -n "$GIT_PUSH_OPTION_COUNT" ] && [ "$GIT_PUSH_OPTION_COUNT" -gt 0 ]; then
    i=0
    while [ $i -lt $GIT_PUSH_OPTION_COUNT ]; do
      eval "opt=\$GIT_PUSH_OPTION_$i"
      echo "$opt"
      i=$((i+1))
    done
  fi
} > "$GIT_DIR/push-options"
`
	return os.WriteFile(
		filepath.Join(hooksDir, "post-receive"),
		[]byte(script), 0o755,
	)
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
