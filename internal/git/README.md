# Git Service — Architecture & User Flow
--- 
## Overall Description
---
This API allows participants to make work on their study projects using our git server and also register attempts.
- In CLI, participants can use command `git push -o "submit"` and so, their last commit becomes an attempt that can be further revied by teacher.
- In web version, participants can upload their files to make an attempt.
## Full User Flow in CLI 
---
```mermaid
flowchart TD
  subgraph Client["Client"]
    A1[git clone / git push<br/>ssh://host:2222/course/task]
  end

  subgraph SSH["SSH Server :2222"]
    B1[charmbracelet/wish]
    B2[PublicKeyAuth / PasswordAuth]
    B3[pkg/git Middleware]
  end

  subgraph MW["pkg/git Middleware<br/>(per session)"]
    C1[repoRename cmd[1], pk<br/>course/task → hash.git]
    C2[gh.AuthRepo repo, pk<br/>check access in ssh_keys]
    C3{switch gc}
    C4{switch access}
    C5[gitPack: git receive-pack<br/>with advertisePushOptions]
    C6[post-receive hook<br/>writes push-options file]
    C7[gh.Push cmd[1], pk]
    C8[gitPack: git upload-pack]
    C9[gh.Fetch repo, pk]
  end

  subgraph Service["internal/git Service<br/>Push callback"]
    D1[Read push-options file]
    D2{Has attempt=confirm?}
    D3[Open bare repo via go-git]
    D4[Read HEAD commit hash]
    D5[repo_db.SaveAttempt]
  end

  subgraph DB["PostgreSQL"]
    E1[ssh_keys: fingerprint → owner_id]
    E2[courses / blocks: name → ID]
    E3[INSERT attempts + attempt_transitions]
  end

  A1 -->|SSH connect & auth| B1
  B1 --> B2
  B2 -->|authenticated session| B3
  B3 --> C1
  C1 -.->|course/task name→ID| E2
  C1 --> C2
  C2 -.->|fingerprint lookup| E1
  C2 --> C3

  C3 -->|git-receive-pack| C4
  C4 -->|ReadWrite / Admin| C5
  C4 -->|NoAccess| F1[Fatal: ErrNotAuthed]
  C5 -->|inside system git| C6
  C6 -->|gitPack returns| C7
  C7 --> D1

  C3 -->|git-upload-pack /<br/>git-upload-archive| C4b{switch access}
  C4b -->|ReadOnly / ReadWrite / Admin| C8
  C4b -->|NoAccess| F1
  C8 -->|gitPack returns| C9
  C9 --> F2[Log fetch event]

  D1 --> D2
  D2 -->|no| F3[Return: no attempt]
  D2 -->|yes| D3
  D3 --> D4
  D4 --> D5
  D5 --> E3
```
