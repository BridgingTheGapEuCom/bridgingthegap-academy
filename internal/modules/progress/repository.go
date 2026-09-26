package progress

import "context"

type Repository interface {
	Create(context.Context, string, string) (CourseProgress, error)
	Get(context.Context, string, string) (CourseProgress, error)
	MarkLessonCompleted(context.Context, string, string, string, int64) (CourseProgress, error)
}
