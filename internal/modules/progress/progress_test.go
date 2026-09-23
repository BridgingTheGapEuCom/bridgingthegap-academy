package progress

import (
	"testing"
	"time"
)

func TestCompletionIsMonotonicAndIdempotent(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p, e := New("10000000-0000-4000-8000-000000000001", "20000000-0000-4000-8000-000000000001", now)
	if e != nil {
		t.Fatal(e)
	}
	q, changed, e := p.MarkLessonCompleted("lesson-a", now.Add(time.Second))
	if e != nil || !changed || q.Revision != 2 {
		t.Fatal(q, changed, e)
	}
	r, changed, e := q.MarkLessonCompleted("lesson-a", now.Add(2*time.Second))
	if e != nil || changed || r.Revision != 2 {
		t.Fatal(r, changed, e)
	}
	if !IsCourseComplete([]string{"lesson-a"}, r) || IsCourseComplete(nil, r) {
		t.Fatal("completion derivation")
	}
}
