package routes

import (
	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/git"
	"github.com/go-fuego/fuego"
)

type GitController struct {
	service *git.Service
}

func NewGitController(svc *git.Service) *GitController {
	return &GitController{
		svc,
	}
}

func (svc *GitController) AddSshKey(
	ctx fuego.ContextWithBody[git.AddSshKey],
) (any, error) {
	body, err := ctx.Body()
	sessionId := session.UserIDFromContext(ctx.Context())
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return nil, svc.service.AddSshKey(sessionId, &body)
}

func (svc *GitController) DeleteSshKey(
	ctx fuego.ContextWithBody[git.DeleteSshKey],
) (any, error) {
	body, err := ctx.Body()
	sessionId := session.UserIDFromContext(ctx.Context())
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return nil, svc.service.DeleteSshKey(sessionId, &body)
}
