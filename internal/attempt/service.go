package attempt

type Service struct {
	RepoManager
	Repo
}

func NewService(repo RepoManager, repo2 Repo) *Service {
	return &Service{repo, repo2}
}
