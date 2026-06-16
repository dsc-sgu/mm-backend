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

    C->>SSH: `git push -o "submit" ssh://host:2222/course_name/task_name`
    SSH->>MW: authenticated session
    MW->>DB: looks for course and task IDs
    DB-->>MW: return course and task IDs
    MW->>MW: RepoRename makes repo name from course ID, task ID and fingerprint
    MW->>MW: AuthRepo checks acces via fingerprint
    MW->>DB: gives fingerprint     
    DB-->>MW: access level
    alt ReadWrite or Admin access
        MW->>Git: user uses `git push` that transforms to `git-receive-pack`    
        Git->>Git: post-receive git hook writes push-options to file
        Git-->>MW: git packfile exchange done
        MW->>Svc: Push hook triggers
        Svc->>Svc: Push hook read push-options file
        alt "submit" appears in push-options
            Svc->>Svc: opens bare repo via go-git
            Svc->>Svc: reads HEAD commit hash
            Svc->>DB: registers attempt
            DB-->>Svc: saved
            Svc-->>MW: attempt created
        else no "submit"
            Svc-->>MW: no attempt
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
    MW->>DB: looks for course and task IDs
    DB-->>MW: return course and task IDs
    MW->>MW: RepoRename makes repo name from course ID, task ID and fingerprint
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
