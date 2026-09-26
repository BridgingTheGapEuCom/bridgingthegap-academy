package community

import (
	"testing"
	"time"
)

const cid = "10000000-0000-4000-8000-000000000001"
const uid = "20000000-0000-4000-8000-000000000001"
const tid = "30000000-0000-4000-8000-000000000001"
const pid = "40000000-0000-4000-8000-000000000001"

func TestCommunityPlainTextValidation(t *testing.T) {
	now := time.Now().UTC()
	if (CourseCommunity{CourseID: cid, Mode: CommunityEnabled, CreatedAt: now, UpdatedAt: now}).Validate() != nil {
		t.Fatal("community")
	}
	if (Thread{ID: tid, CourseID: cid, Title: " A title ", CreatedByUserID: uid, State: Visible, CreatedAt: now, UpdatedAt: now}).Validate() == nil {
		t.Fatal("un-normalized title")
	}
	title, e := NormalizeTitle(" A title \n")
	if e != nil || title != "A title" {
		t.Fatal(title, e)
	}
	if _, e := NormalizeBody(" \n "); e == nil {
		t.Fatal("blank body")
	}
	if (Post{ID: pid, ThreadID: tid, AuthorUserID: uid, Body: "line one\nline two", State: Visible, CreatedAt: now, UpdatedAt: now}).Validate() != nil {
		t.Fatal("post")
	}
}
