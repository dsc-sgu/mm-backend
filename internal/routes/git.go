package routes

import (
	"github.com/go-fuego/fuego"

	"github.com/dsc-sgu/mm-backend/internal/auth/session"
	"github.com/dsc-sgu/mm-backend/internal/git"
)

type GitController struct {
	service *git.Service
}

func NewGitController(svc *git.Service) *GitController {
	return &GitController{
		svc,
	}
}

func (svc *GitController) AddSSHKey(
	ctx fuego.ContextWithBody[git.AddSSHKey],
) (any, error) {
	body, err := ctx.Body()
	sessionID := session.UserIDFromContext(ctx.Context())
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return nil, svc.service.AddSSHKey(sessionID, &body)
}

func (svc *GitController) DeleteSSHKey(
	ctx fuego.ContextWithBody[git.DeleteSSHKey],
) (any, error) {
	body, err := ctx.Body()
	sessionID := session.UserIDFromContext(ctx.Context())
	if err != nil {
		return nil, fuego.BadRequestError{Title: "INVALID_JSON"}
	}

	return nil, svc.service.DeleteSSHKey(sessionID, &body)
}
