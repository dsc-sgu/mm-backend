package git

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Hook names, which are also the arguments the hook scripts pass to the
// server binary: "<server> hook <name>".
const (
	PreReceiveHook  = "pre-receive"
	PostReceiveHook = "post-receive"
)

// Environment the SSH server passes to git receive-pack, which passes it on
// to the hooks it runs.
const (
	EnvHookSocket    = "MM_HOOK_SOCKET"
	EnvCourseID      = "MM_COURSE_ID"
	EnvTaskGroupID   = "MM_TASK_GROUP_ID"
	EnvParticipantID = "MM_PARTICIPANT_ID"
)

// ZeroHash is the object name git gives the missing side of a ref update.
const ZeroHash = "0000000000000000000000000000000000000000"

// HookRef is one ref update git hands a hook on stdin. For a new tag the
// pre-receive hook also lists the files of its commit: until the push is
// accepted, the pushed objects are quarantined and only git and its hooks
// can read them.
type HookRef struct {
	Old   string   `json:"old"`
	New   string   `json:"new"`
	Ref   string   `json:"ref"`
	Files []string `json:"files,omitempty"`
}

// IsNewTag reports whether the update creates a tag.
func (r HookRef) IsNewTag() bool {
	return strings.HasPrefix(r.Ref, "refs/tags/") && r.Old == ZeroHash
}

// HookRequest is what a hook sends to the server. Task is the value of the
// last "-o submit=<task>" push option, empty if there is none.
type HookRequest struct {
	Repo RepoID    `json:"repo"`
	Task string    `json:"task"`
	Refs []HookRef `json:"refs"`
}

// HookEnv returns the environment that lets the hooks of a push to id reach
// the server listening on socket.
func HookEnv(socket string, id RepoID) []string {
	return []string{
		EnvHookSocket + "=" + socket,
		EnvCourseID + "=" + id.CourseID.String(),
		EnvTaskGroupID + "=" + id.TaskGroupID.String(),
		EnvParticipantID + "=" + id.ParticipantID.String(),
	}
}

// WriteHooks writes the pre-receive and post-receive hooks of the repository
// at repoPath. Each hook runs exe as "exe hook <name>".
func WriteHooks(repoPath, exe string) error {
	hooksDir := filepath.Join(repoPath, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	quoted := "'" + strings.ReplaceAll(exe, "'", `'\''`) + "'"
	for _, name := range []string{PreReceiveHook, PostReceiveHook} {
		script := fmt.Sprintf("#!/bin/sh\nexec %s hook %s\n", quoted, name)
		if err := os.WriteFile(
			filepath.Join(hooksDir, name),
			[]byte(script), 0o755,
		); err != nil {
			return fmt.Errorf("write %s hook: %w", name, err)
		}
	}
	return nil
}

// RunHook is the hook process. It reads the ref updates git passes on stdin,
// sends them to the server and prints the server's refusal to stderr, which
// git relays to the client. It returns the hook's exit code.
func RunHook(name string, stdin io.Reader, stderr io.Writer) int {
	if err := runHook(name, stdin); err != nil {
		_, _ = fmt.Fprintf(stderr, "ERROR: %s\n", err)
		return 1
	}
	return 0
}

func runHook(name string, stdin io.Reader) error {
	if name != PreReceiveHook && name != PostReceiveHook {
		return fmt.Errorf("unknown hook %q", name)
	}
	repo, err := repoFromEnv()
	if err != nil {
		return err
	}
	request := HookRequest{Repo: repo, Task: submittedTask()}

	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 3 {
			continue
		}
		ref := HookRef{Old: fields[0], New: fields[1], Ref: fields[2]}
		if name == PreReceiveHook && ref.IsNewTag() {
			if ref.Files, err = commitFiles(ref.New); err != nil {
				return err
			}
		}
		request.Refs = append(request.Refs, ref)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read ref updates: %w", err)
	}

	return postHook(os.Getenv(EnvHookSocket), name, request)
}

func repoFromEnv() (RepoID, error) {
	var id RepoID
	for env, target := range map[string]*uuid.UUID{
		EnvCourseID:      &id.CourseID,
		EnvTaskGroupID:   &id.TaskGroupID,
		EnvParticipantID: &id.ParticipantID,
	} {
		value, err := uuid.Parse(os.Getenv(env))
		if err != nil {
			return RepoID{}, fmt.Errorf("%s: %w", env, err)
		}
		*target = value
	}
	return id, nil
}

// submittedTask returns the value of the last "submit=<task>" push option.
func submittedTask() string {
	count, _ := strconv.Atoi(os.Getenv("GIT_PUSH_OPTION_COUNT"))
	options := make([]string, 0, count)
	for i := range count {
		options = append(options, os.Getenv(fmt.Sprintf("GIT_PUSH_OPTION_%d", i)))
	}
	return parseSubmitOption(options)
}

func parseSubmitOption(options []string) string {
	task := ""
	for _, option := range options {
		if value, ok := strings.CutPrefix(
			strings.TrimSpace(option),
			"submit=",
		); ok {
			task = strings.TrimSpace(value)
		}
	}
	return task
}

// commitFiles lists the files of the commit object names. The hook runs in
// the repository with git's quarantine environment, so git sees the pushed
// objects.
func commitFiles(object string) ([]string, error) {
	out, err := exec.Command(
		"git", "ls-tree", "-r", "-z", "--name-only", object,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("list files of %s: %w", object, err)
	}
	var files []string
	for _, file := range bytes.Split(out, []byte{0}) {
		if len(file) > 0 {
			files = append(files, string(file))
		}
	}
	return files, nil
}

// postHook sends request to the hook endpoint name of the server listening
// on socket. A non-2xx answer carries the reason as plain text.
func postHook(socket, name string, request HookRequest) error {
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	client := http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		},
	}}
	resp, err := client.Post(
		"http://hook/"+name,
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("reach server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 == 2 {
		return nil
	}
	reason, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s", strings.TrimSpace(string(reason)))
}
