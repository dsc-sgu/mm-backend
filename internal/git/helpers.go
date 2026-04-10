package git

import (
	"context"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"github.com/dsc-sgu/mm-backend/internal/courses"
)

type CourseController struct {
	courseService *courses.Service
}

func NewCourseController(
	courseService *courses.Service,
) *CourseController {
	return &CourseController{
		courseService: courseService,
	}
}

func (c *CourseController) GetCourse(
	name string,
) (uuid.UUID, error) {
	ctx := context.Background()
	course, err := c.courseService.GetCourseByName(ctx, name)
	if err != nil {
		return uuid.Nil, fuego.InternalServerError{Detail: err.Error()}
	}

	return course.ID, nil
}
