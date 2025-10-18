package disciplines

type Service struct {
	repo Repo
}

func NewService(repo Repo) *Service {
	return &Service{repo}
}
