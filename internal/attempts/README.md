# Attempt API

## Overall description

This API allows course participants to create attempts for tasks within task groups.

### Attempt flow

There are two ways to create an attempt:

1. **Web interface** — upload a ZIP archive with solution files. The backend extracts the files, makes a commit in the group's git repository, and saves the attempt with the commit hash.

2. **SSH git tag push** — create a tag and push it with a numeric push option indicating the task position:
   ```
   git tag submission-1 && git push origin submission-1 -o "1"
   ```
   The pre-receive hook verifies the tag's commit contains files matching the task's patterns. If they match, the post-receive hook saves the tag's commit hash and push options. The Push hook reads these files and creates an attempt for the tag's commit.

### Sequential validation

If a task group has multiple tasks, submitting to task N requires a submitted attempt on task N-1. This applies to both web and SSH pushes.

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
        Student->>Backend: POST /attempts (ZIP + courseID + taskGroupID + taskPosition)
        Backend->>Backend: Resolve task ID from group + position
        Backend->>Git: Push files to group repo (clone/write/commit)
        Git->>DB: Save attempt (user, task, commit hash)
    else SSH tag push
        Student->>Git: git tag v1 && git push origin v1 -o "1"
        Git->>Git: pre-receive hook checks tag's files against patterns
        Git->>Git: post-receive hook saves new tag commit + push-options
        Git->>Git: Push hook reads files, validates sequence
        Git->>DB: Save attempt (user, task, tag's commit hash)
    end
```

## Key points

- One git repository per task group per student (all tasks share the same repo).
- Attempts are linked to individual tasks, not to the whole group.
- The commit hash is stored in `attempt_transitions.transition_data` as JSON.
- Both push methods write to the same repository.
- Tag updates (force-push) are rejected by the pre-receive hook.
- Branch pushes never create attempts — only tag pushes with a numeric option do.
