# Attempt API

## Overall description

This API allows course participants to create attempts for tasks within task groups.

### Attempt flow

There are two ways to create an attempt:

1. **Web interface** — upload a ZIP archive with solution files. The backend extracts the files, makes a commit in the group's git repository, and saves the attempt with the commit hash.

2. **SSH git tag push** — create a tag and push it with the `submit` push option naming the task:
   ```
   git tag submission-1 && git push origin submission-1 -o submit=<task_name>
   ```
   The hooks are the server binary run as `server hook pre-receive` and `server hook post-receive`. They send the ref updates, the task name and, for pre-receive, the files of each new tag's commit to the backend over a unix socket (`HOOK_SOCKET`). The backend decides: pre-receive is rejected if a tag is updated, the task does not exist, a tag's commit has no file matching the task's patterns, or the participant has fewer attempts left than new tags. Post-receive creates an attempt for each new tag.

### Mask gate

A task may define file patterns (`tasks.patterns`). A submission is rejected unless at least one of its files matches. Both paths check it in the backend with the same `CompilePatterns`: the web path in `PushAttempt`, SSH when the pre-receive hook asks.

### Database structure

An attempt (`attempts` table) links a user to a task (`tasks.id`) and has a chain of state transitions (`attempt_transitions` table). The transition data stores the commit hash from the git repository.

## Push Flow

```mermaid
sequenceDiagram
    actor Student
    participant Backend
    participant Git as Git Service
    participant DB as Database

    alt Web upload
        Student->>Backend: POST /attempts (ZIP + courseID + taskGroupID + taskID)
        Backend->>Backend: Validate task belongs to the group
        Backend->>Backend: Mask gate — files must match task patterns (Go)
        Backend->>Git: Push files to group repo (clone/write/commit)
        Git->>DB: Save attempt (user, task, commit hash)
    else SSH tag push
        Student->>Git: git tag v1 && git push origin v1 -o submit=<task_name>
        Git->>Backend: pre-receive hook sends new tags, their files and the task name
        Backend->>Backend: Task exists, mask gate, attempts left
        Git->>Backend: post-receive hook sends new tags
        Backend->>DB: Save attempt (user, task, tag's commit hash)
    end
```

## Key points

- One git repository per task group per student (all tasks share the same repo).
- Attempts are linked to individual tasks, not to the whole group.
- The commit hash is stored in `attempt_transitions.transition_data` as JSON.
- Both push methods write to the same repository.
- Tag updates (force-push) are rejected when the pre-receive hook asks the backend.
- Branch pushes never create attempts — only tag pushes with `-o submit=<task_name>` do.
