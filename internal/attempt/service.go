package attempt

type Service struct {
	RepoManager
}

func NewService(repo RepoManager) *Service {
	return &Service{repo}
}
