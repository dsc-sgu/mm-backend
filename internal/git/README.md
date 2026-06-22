# Git Service — Architecture & User Flow

This API allows participants to work on study projects using the built-in Git server and register attempts.
- **CLI**: `git tag <name> && git push origin <name> -o submit=<task_name>` — tags a commit and submits it as an attempt for the named task.
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

    C->>SSH: `git push origin v1 -o submit=<task_name>` (ssh://host:2222/course_name/group_name)
    SSH->>MW: authenticated session
    MW->>DB: looks for course and task group IDs
    DB-->>MW: return course and task group IDs
    MW->>MW: RepoRename makes repo name from course ID, task group ID and fingerprint
    MW->>MW: AuthRepo checks acces via fingerprint
    MW->>DB: gives fingerprint     
    DB-->>MW: access level
    alt ReadWrite or Admin access
        MW->>Git: git-receive-pack runs system git
        Git->>Git: pre-receive hook checks tag files against .mm-patterns (by task name)
        alt no matching files for task
            Git-->>MW: reject push (exit 1)
        else matches or no patterns
            Git->>Git: post-receive hook writes push-tags + push-options to files
            Git-->>MW: git packfile exchange done
            MW->>Svc: OnPush hook triggers
            Svc->>Svc: read push-options (submit task name)
            Svc->>Svc: read push-tags (new tag commit hashes)
            Svc->>DB: resolve task by name
            Svc->>DB: register attempt for each new tag
            DB-->>Svc: saved
        end
    else NoAccess
        MW-->>C: Fatal error
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
    MW->>DB: looks for course and task group IDs
    DB-->>MW: return course and task group IDs
    MW->>MW: RepoRename makes repo name from course ID, task group ID and fingerprint
    MW->>MW: AuthRepo checks acces via fingerprint
    MW->>DB: gives fingerprint     
    DB-->>MW: access level
    alt ReadOnly or ReadWrite or Admin
        MW->>Git: user uses `git clone` that transforms to `git-upload-pack`        
        Git-->>MW: git packfile exchange done
        MW->>Svc: Fetch hook triggers (just logging)
    else NoAccess
        MW-->>C: Fatal: ErrNotAuthed
    end
```
