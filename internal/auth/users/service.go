package users

type Service struct {
	Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}
