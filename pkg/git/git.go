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

// Package git serves Git repositories over SSH with go-git's in-process
// upload-pack and receive-pack, enforcing the push rules of task submissions,
// and manages the repositories on disk (see Manager).
package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp"
	"github.com/go-git/go-git/v6/plumbing/revlist"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/utils/ioutil"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	// ErrNotAuthed represents unauthorized access.
	ErrNotAuthed = errors.New("you are not authorized to do this")
	// ErrSystemMalfunction represents a general system error returned to clients.
	ErrSystemMalfunction = errors.New("something went wrong")
	// ErrInvalidRepo represents an attempt to access a non-existent repo.
	ErrInvalidRepo = errors.New("invalid repo")
	// ErrPushRejected wraps every reason a push is refused; the client sees
	// its text.
	ErrPushRejected = errors.New("push rejected")
)

type GitCmd string

const (
	GitUploadPack  GitCmd = "git-upload-pack"
	GitReceivePack GitCmd = "git-receive-pack"
)

// Task is a task of a task group that a tag push can submit.
type Task struct {
	ID       uuid.UUID
	Name     string
	Patterns []string
	// MaxAttempts is nil when attempts are not limited.
	MaxAttempts *int
}

// Store is what serving course repositories over SSH needs from the database.
type Store interface {
	// RepoForPath resolves a course and a task group by name and the owner
	// of the SSH key with fingerprint to their repository, returning
	// ErrInvalidRepo when there is no such course, group or key.
	RepoForPath(ctx context.Context, course, group, fingerprint string) (RepoID, error)
	IsCourseMember(ctx context.Context, userID, courseID uuid.UUID) (bool, error)
	// GetLiveTasks lists the tasks of a task group in its course's active
	// snapshot.
	GetLiveTasks(ctx context.Context, taskGroupID uuid.UUID) ([]Task, error)
	// CountAttempts tells how many attempts participantID has made at taskID.
	CountAttempts(ctx context.Context, taskID, participantID uuid.UUID) (int, error)
	SaveAttempt(repoID RepoID, taskID uuid.UUID, commitHash string) error
}

// ResolveRepo resolves an SSH repository path "course/group[.git]" for the
// owner of the key with fingerprint and checks that they are a member of the
// course. The error wraps ErrInvalidRepo when the path names no task group of
// a course and ErrNotAuthed when the owner is not a member.
func ResolveRepo(ctx context.Context, store Store, path, fingerprint string) (RepoID, error) {
	course, group, ok := strings.Cut(strings.Trim(strings.TrimSuffix(path, ".git"), "/"), "/")
	if !ok || strings.Contains(group, "/") {
		return RepoID{}, fmt.Errorf("%w: %q is not course/group", ErrInvalidRepo, path)
	}
	id, err := store.RepoForPath(ctx, course, group, fingerprint)
	if err != nil {
		return RepoID{}, err
	}
	member, err := store.IsCourseMember(ctx, id.ParticipantID, id.CourseID)
	if err != nil {
		return RepoID{}, err
	}
	if !member {
		return RepoID{}, ErrNotAuthed
	}
	return id, nil
}

// GitPack serves cmd for repository id, stored at repoPath, over s. A push is
// checked by the push rules (see checkPush) and every tag it creates is saved
// through store as an attempt.
func GitPack(s ssh.Session, cmd GitCmd, repoPath string, id RepoID, store Store) error {
	repo, err := gogit.PlainOpen(repoPath)
	if err != nil {
		return err
	}
	// UploadPack closes both streams once the pack is sent. Closing the
	// session there would drop it before its exit status is sent, so the
	// client would fail every non-empty clone or fetch.
	r, w := io.NopCloser(s), ioutil.WriteNopCloser(s)
	switch cmd {
	case GitUploadPack:
		return transport.UploadPack(s.Context(), repo.Storer, r, w, &transport.UploadPackRequest{})
	case GitReceivePack:
		hooks := receiveHooks(id, store)
		return transport.ReceivePack(s.Context(), repo.Storer, r, w, &transport.ReceivePackRequest{Hooks: hooks})
	default:
		return fmt.Errorf("unknown git command: %s", cmd)
	}
}

// Fatal prints to the session's STDOUT as a git response and exit 1.
func Fatal(s ssh.Session, err error) {
	msg := "ERR " + err.Error()
	// hex length includes 4 byte length prefix and ending newline
	_, _ = wish.WriteString(s, fmt.Sprintf("%04x%s\n", len(msg)+5, msg))
	s.Exit(1) // nolint: errcheck
}

// receiveHooks checks a push to repository id before any ref moves and, once
// the refs moved, saves each created tag as an attempt at the task the check
// settled on.
func receiveHooks(id RepoID, store Store) transport.ReceivePackHooks {
	var (
		taskID  uuid.UUID
		commits map[plumbing.ReferenceName]plumbing.Hash
	)
	return transport.ReceivePackHooks{
		PreReceive: func(ctx context.Context, info *transport.PreReceiveInfo) error {
			var err error
			taskID, commits, err = checkPush(ctx, id, store, info)
			// go-git sends the error text to the client, so only refusals
			// may reach it as they are.
			if err != nil && !errors.Is(err, ErrPushRejected) {
				zap.L().Error("check push", zap.Error(err))
				return fmt.Errorf("%w: %w", ErrPushRejected, ErrSystemMalfunction)
			}
			return err
		},
		PostReceive: func(_ context.Context, info *transport.PostReceiveInfo) error {
			if err := ensureDefaultBranch(info.Storer, info.Commands); err != nil {
				zap.L().Warn("point HEAD at pushed branch", zap.Error(err))
			}
			for _, cmd := range info.Commands {
				commit, ok := commits[cmd.Name]
				if !ok {
					continue
				}
				tag := cmd.Name.Short()
				if err := store.SaveAttempt(id, taskID, commit.String()); err != nil {
					zap.L().Error("save pushed attempt", zap.Error(err), zap.String("tag", tag))
					_, _ = fmt.Fprintf(info.Progress, "submission %s was NOT recorded, push a new tag to retry\n", tag)
					continue
				}
				_, _ = fmt.Fprintf(info.Progress, "submission %s recorded\n", tag)
			}
			return nil
		},
	}
}

// checkPush enforces the push rules: branches only move forward and are never
// deleted; tags are only created; every pushed commit has all its objects;
// every tag submits one task (the one named by the single "submit=<task>"
// push option, or the group's only task when the option is left out) and
// points to a commit holding a file that matches the task's patterns; nothing
// else can be pushed. It returns the submitted task and the commit of every
// created tag.
func checkPush(
	ctx context.Context,
	id RepoID,
	store Store,
	info *transport.PreReceiveInfo,
) (uuid.UUID, map[plumbing.ReferenceName]plumbing.Hash, error) {
	name, err := parseSubmitOption(info.PushOptions)
	if err != nil {
		return uuid.Nil, nil, err
	}
	known, err := refTips(info.Storer)
	if err != nil {
		return uuid.Nil, nil, err
	}
	var tags []*packp.Command
	for _, cmd := range info.Commands {
		switch {
		case cmd.Name.IsBranch():
			if err := checkBranch(info.Storer, cmd, known); err != nil {
				return uuid.Nil, nil, err
			}
		case cmd.Name.IsTag() && cmd.Action() == packp.Create:
			tags = append(tags, cmd)
		case cmd.Name.IsTag():
			return uuid.Nil, nil, fmt.Errorf("%w: tag %s cannot be moved or deleted", ErrPushRejected, cmd.Name.Short())
		default:
			return uuid.Nil, nil, fmt.Errorf("%w: only branches and tags can be pushed", ErrPushRejected)
		}
	}
	if len(tags) == 0 {
		if name != "" {
			return uuid.Nil, nil, fmt.Errorf("%w: -o submit needs a new tag to submit", ErrPushRejected)
		}
		return uuid.Nil, nil, nil
	}

	tasks, err := store.GetLiveTasks(ctx, id.TaskGroupID)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("get tasks: %w", err)
	}
	i := slices.IndexFunc(tasks, func(t Task) bool { return t.Name == name })
	if name == "" && len(tasks) == 1 {
		i = 0
	}
	if i < 0 {
		names := make([]string, len(tasks))
		for j, task := range tasks {
			names[j] = task.Name
		}
		switch {
		case len(tasks) == 0:
			return uuid.Nil, nil, fmt.Errorf("%w: this task group has no tasks to submit", ErrPushRejected)
		case name == "":
			return uuid.Nil, nil, fmt.Errorf(
				"%w: name the task a tag submits with -o submit=<task>, one of %q", ErrPushRejected, names,
			)
		default:
			return uuid.Nil, nil, fmt.Errorf("%w: no task named %q, tasks are %q", ErrPushRejected, name, names)
		}
	}
	task := tasks[i]

	if task.MaxAttempts != nil {
		used, err := store.CountAttempts(ctx, task.ID, id.ParticipantID)
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("count attempts: %w", err)
		}
		if used+len(tags) > *task.MaxAttempts {
			return uuid.Nil, nil, fmt.Errorf("%w: no attempts left for task '%s'", ErrPushRejected, task.Name)
		}
	}

	compiledPatterns := CompilePatterns(task.Patterns)
	commits := make(map[plumbing.ReferenceName]plumbing.Hash, len(tags))
	for _, cmd := range tags {
		tag := cmd.Name.Short()
		// A lightweight tag names the commit itself, an annotated one a tag
		// object pointing to it.
		commit, err := object.GetCommit(info.Storer, cmd.New)
		if annotated, tagErr := object.GetTag(info.Storer, cmd.New); tagErr == nil {
			commit, err = annotated.Commit()
		}
		if err != nil {
			return uuid.Nil, nil, fmt.Errorf("%w: tag %s does not point to a commit", ErrPushRejected, tag)
		}
		if err := checkConnected(info.Storer, cmd, known); err != nil {
			return uuid.Nil, nil, err
		}
		if len(task.Patterns) > 0 {
			files, err := commit.Files()
			if err != nil {
				return uuid.Nil, nil, fmt.Errorf("list files of tag %s: %w", tag, err)
			}
			matched := false
			err = files.ForEach(func(f *object.File) error {
				if compiledPatterns.MatchAny(f.Name) {
					matched = true
					return storer.ErrStop
				}
				return nil
			})
			if err != nil {
				return uuid.Nil, nil, fmt.Errorf("list files of tag %s: %w", tag, err)
			}
			if !matched {
				return uuid.Nil, nil, fmt.Errorf(
					"%w: tag %s has no files matching %v", ErrPushRejected, tag, task.Patterns,
				)
			}
		}
		commits[cmd.Name] = commit.Hash
	}
	return task.ID, commits, nil
}

// checkBranch refuses deleting a branch, pointing it at anything but a
// commit, and moving it to a commit that does not descend from its current
// one, so saved attempts stay reachable and the branch stays usable for web
// uploads.
func checkBranch(st storer.EncodedObjectStorer, cmd *packp.Command, known []plumbing.Hash) error {
	branch := cmd.Name.Short()
	if cmd.Action() == packp.Delete {
		return fmt.Errorf("%w: deleting branch %s is not allowed", ErrPushRejected, branch)
	}
	newCommit, err := object.GetCommit(st, cmd.New)
	if err != nil {
		return fmt.Errorf("%w: branch %s must point to a commit", ErrPushRejected, branch)
	}
	if err := checkConnected(st, cmd, known); err != nil {
		return err
	}
	if cmd.Action() != packp.Update {
		return nil
	}
	oldCommit, err := object.GetCommit(st, cmd.Old)
	if err != nil {
		return fmt.Errorf("read branch %s: %w", branch, err)
	}
	fastForward, err := oldCommit.IsAncestor(newCommit)
	if err != nil {
		return fmt.Errorf("check branch %s history: %w", branch, err)
	}
	if !fastForward {
		return fmt.Errorf("%w: force-pushing branch %s is not allowed", ErrPushRejected, branch)
	}
	return nil
}

// checkConnected refuses a pushed ref whose history or trees miss objects:
// unlike git, go-git does not check that a pushed pack is complete. known are
// the tips the repository already has, whose objects need no check.
func checkConnected(st storer.EncodedObjectStorer, cmd *packp.Command, known []plumbing.Hash) error {
	objects, err := revlist.Objects(st, []plumbing.Hash{cmd.New}, known)
	for _, h := range objects {
		if err != nil {
			break
		}
		err = st.HasEncodedObject(h)
	}
	if err != nil {
		return fmt.Errorf("%w: %s is incomplete: %v", ErrPushRejected, cmd.Name.Short(), err)
	}
	return nil
}

// refTips lists the objects the repository's refs point to.
func refTips(st storer.ReferenceStorer) ([]plumbing.Hash, error) {
	refs, err := st.IterReferences()
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}
	var tips []plumbing.Hash
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() == plumbing.HashReference {
			tips = append(tips, ref.Hash())
		}
		return nil
	})
	return tips, err
}

// parseSubmitOption returns the task named by the push's "submit=<task>"
// option, or "" when there is none. Options are matched exactly, without
// trimming, and more than one submit option is refused.
func parseSubmitOption(options []string) (string, error) {
	name := ""
	for _, option := range options {
		value, ok := strings.CutPrefix(option, "submit=")
		if !ok {
			continue
		}
		if name != "" {
			return "", fmt.Errorf("%w: only one -o submit=<task> option is allowed", ErrPushRejected)
		}
		if value == "" {
			return "", fmt.Errorf("%w: -o submit= needs a task name", ErrPushRejected)
		}
		name = value
	}
	return name, nil
}

// ensureDefaultBranch points an unborn HEAD at the first branch of a push, so
// a repository whose first push is not "master" still has a default branch.
func ensureDefaultBranch(st storer.ReferenceStorer, cmds []*packp.Command) error {
	if _, err := storer.ResolveReference(st, plumbing.HEAD); !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return err
	}
	for _, cmd := range cmds {
		if cmd.Name.IsBranch() && cmd.Action() != packp.Delete {
			return st.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, cmd.Name))
		}
	}
	return nil
}
