# Git Service — Architecture & User Flow

This API allows participants to work on study projects using the built-in Git server and register attempts.
- **CLI**: `git push -o "submit"` — the last commit becomes an attempt for teacher review.
- **Web UI**: participants upload files to create an attempt.

## Push Flow

```mermaid
sequenceDiagram
    participant C as Client
    participant SSH as SSH Server (wish)
    participant MW as pkg/git Middleware
    participant Svc as internal/git Service
    participant Git as system git
    participant DB as PostgreSQL

    C->>SSH: git push -o "submit" ssh://host:2222/course/task
    SSH->>MW: authenticated session
    MW->>MW: repoRename(course/task → hash.git)
    MW->>DB: lookup course/task → IDs
    DB-->>MW: course_id, task_id
    MW->>MW: AuthRepo(repo, publicKey)
    MW->>DB: fingerprint → owner_id, access
    DB-->>MW: access level
    alt ReadWrite or Admin
        MW->>Git: git receive-pack (advertisePushOptions=true)
        Git->>Git: post-receive hook → push-options file
        Git-->>MW: packfile exchange done
        MW->>Svc: Push callback
        Svc->>Svc: read push-options file
        alt "submit" in options
            Svc->>Svc: open bare repo via go-git
            Svc->>Svc: read HEAD commit hash
            Svc->>DB: INSERT attempt + attempt_transitions
            DB-->>Svc: saved
            Svc-->>MW: attempt created
        else no "submit"
            Svc-->>MW: no attempt
        end
    else NoAccess
        MW-->>C: Fatal: ErrNotAuthed
    end
```

## Fetch Flow

```mermaid
sequenceDiagram
    participant C as Client
    participant SSH as SSH Server (wish)
    participant MW as pkg/git Middleware
    participant Svc as internal/git Service
    participant Git as system git
    participant DB as PostgreSQL

    C->>SSH: git clone/fetch ssh://host:2222/course/task
    SSH->>MW: authenticated session
    MW->>MW: repoRename(course/task → hash.git)
    MW->>DB: lookup course/task → IDs
    DB-->>MW: course_id, task_id
    MW->>MW: AuthRepo(repo, publicKey)
    MW->>DB: fingerprint → owner_id, access
    DB-->>MW: access level
    alt ReadOnly or ReadWrite or Admin
        MW->>Git: git upload-pack / git-upload-archive
        Git-->>MW: packfile exchange done
        MW->>Svc: Fetch callback
        Svc-->>DB: log fetch event
    else NoAccess
        MW-->>C: Fatal: ErrNotAuthed
    end
```
