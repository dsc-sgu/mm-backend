package dto

type CoursePagination struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}
