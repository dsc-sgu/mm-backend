# Git server implementation

## Overall description

This module provides custom SSH server which allows to inject reaction on push/fetch events. You can write implementation of `Hooks` interface and inject it into `Middleware` to get any custom logic you want.

We intend to use `Middleware` with `Hooks` to:
1. Authorize `git-recieve-pack` write access for student (internal part of `git push`).
2. Authorize `git-upload-pack` (internal part of `git fetch`) or `git-upload-archive` (internal part of `git archive`) read access for teacher.

You *may not* understand internal parts of this package and *it's okay* to.

## Use-cases

### git push

```mermaid
sequenceDiagram
    actor Student
    participant Middleware as pkg/git Middleware
    participant Hooks as App (Hooks)

    Student->>Middleware: git push ssh://server/course/group
    Middleware->>Hooks: RepoRename("/course/group", pubkey) → "abc123.git"
    Middleware->>Hooks: AuthRepo("/course/group", "abc123.git", pubkey) → ReadWriteAccess
    Middleware->>Student: git protocol (packfile exchange)
    Middleware->>Hooks: OnPush("/course/group", "abc123.git", pubkey)
```

### git fetch

```mermaid
sequenceDiagram
    actor Teacher
    participant Middleware as pkg/git Middleware
    participant Hooks as App (Hooks)

    Teacher->>Middleware: git fetch ssh://server/course/group
    Middleware->>Hooks: RepoRename("/course/group", pubkey) → "abc123.git"
    Middleware->>Hooks: AuthRepo("/course/group", "abc123.git", pubkey) → ReadOnlyAccess
    Middleware->>Teacher: git protocol (packfile exchange)
    Middleware->>Hooks: OnFetch("/course/group", "abc123.git", pubkey)
```