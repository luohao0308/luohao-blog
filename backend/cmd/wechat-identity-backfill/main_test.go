package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data"
)

func TestSafeBackfillError(t *testing.T) {
	for _, tc := range []struct {
		err      error
		category string
	}{
		{data.ErrWechatBackfillCommitUnknown, "backfill_commit_outcome_unknown"},
		{biz.ErrUserWechatConflict, "backfill_identity_conflict"},
		{errors.New("private-dsn-and-openid"), "backfill_storage_or_validation_failure"},
	} {
		got := safeBackfillError(tc.err).Error()
		if !strings.HasPrefix(got, tc.category) || strings.Contains(got, "private-dsn") || strings.Contains(got, "no transaction committed") {
			t.Fatal("unsafe CLI outcome", got)
		}
	}
}
