# Attempt API

## Overall description

This API allows to make attempt for course participants.

### Attempt flow

In general, there're two ways to create an attempt:
1. Send files via UI interface (our frontend)
2. Push attempt directly via Git (exact way is not designed yet and implentation is abstraced via `RepoManager` interface).

## Main concepts

### `MakeAttempt` high-level request flow
```mermaid
sequenceDiagram
    title Single request flow
    actor Client
    participant UI
    participant Backend
    participant RepoManager
    participant Git Server
    participant Database
    alt UI way
    Client ->> UI: Uploads file via interface
    UI->>Backend: Sends `MakeAttempt` request
    Backend -->> RepoManager: Calls `PushAttempt`
    Backend ->> UI: Sends confirmation that push is scheduled
    RepoManager ->> Git Server: Runs some push  logic (new commits + tag)
    else Git way
    Client ->> Git Server: Pushs commits directly
    end
    Git Server ->> Backend: Calls internal attempt API with additional Git metadata
    Backend ->> Database: Persist data about attempt
```
