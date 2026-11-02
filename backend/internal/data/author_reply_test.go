package data

import (
	"context"
	"errors"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/article"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/comment"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/wechatcommentcheck"
	"sync"
	"testing"
	"time"
)

func TestAuthorReplyLifecycle(t *testing.T)      { authorReplyLifecycle(t) }
func TestAuthorReplyMySQLLifecycle(t *testing.T) { miniMySQL(t); authorReplyLifecycle(t) }
func authorReplyLifecycle(t *testing.T) {
	ctx := context.Background()
	mini, db, actor := miniFixture(t)
	_, err := db.User.UpdateOneID(actor.UserID).SetRole(biz.UserRoleAdmin).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	exists, e := db.Article.Query().Where(article.SlugEQ("post")).Exist(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !exists {
		a, e := db.Article.Create().SetSlug("post").SetTitle("fixture").SetStatus(biz.ArticleStatusPublished).Save(ctx)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { _ = db.Article.DeleteOneID(a.ID).Exec(ctx) })
	}
	root := miniPass(t, mini, miniCreate(t, mini, actor, "root", "根评论"), true)
	repo := NewCommentRepo(&Data{db: db})
	r := repo.(biz.AuthorReplyRepo)
	input := &biz.Comment{Content: "答复 A", DisplayName: "原始昵称", UserID: &actor.UserID, Status: biz.CommentStatusApproved}
	child, err := r.PutAuthorReply(ctx, root.Comment.ID, input, 0)
	if err != nil || child.ReplyVersion != 1 || child.ParentID == nil {
		t.Fatal("create", err)
	}
	t.Cleanup(func() { _ = repo.Delete(ctx, root.Comment.ID) })
	other, e := db.User.Create().SetEmail(t.Name() + "-other@example.invalid").SetPasswordHash("synthetic-only").SetDisplayName("其他管理员").SetRole(biz.UserRoleAdmin).Save(ctx)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = db.User.DeleteOneID(other.ID).Exec(ctx) })
	foreign := *input
	foreign.UserID = &other.ID
	if _, err = r.PutAuthorReply(ctx, root.Comment.ID, &foreign, 1); err != biz.ErrAuthForbidden {
		t.Fatal("other admin impersonated original author", err)
	}
	if _, err = r.PutAuthorReply(ctx, root.Comment.ID, input, 0); err != biz.ErrAuthorReplyConflict {
		t.Fatal("stale create", err)
	}
	roots, err := repo.ListApprovedBySlug(ctx, "post", 100, 0)
	if err != nil || len(roots) != 1 || roots[0].AuthorReply == nil {
		t.Fatal("root pagination", err)
	}
	public, _, err := mini.ListPublic(ctx, "post", actor.AppID, 100, 0)
	if err != nil || len(public) != 1 || public[0].Comment.AuthorReply != nil {
		t.Fatal("unchecked child escaped", err)
	}
	checking, _, err := mini.BeginRecheck(ctx, child.ID, actor, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	passed := miniPass(t, mini, checking, false)
	public, _, err = mini.ListPublic(ctx, "post", actor.AppID, 100, 0)
	if err != nil || len(public) != 1 || public[0].Comment.AuthorReply == nil {
		t.Fatal("checked child missing", err)
	}
	// A passed child is private when its parent or article stops being public.
	if _, err = db.Comment.UpdateOneID(root.Comment.ID).SetStatus(biz.CommentStatusPending).Save(ctx); err != nil {
		t.Fatal(err)
	}
	owned, err := mini.FindOwned(ctx, child.ID, actor.UserID, actor.AppID)
	if err != nil || owned.Visible() {
		t.Fatal("child visible with pending parent", err)
	}
	if _, err = db.Comment.UpdateOneID(root.Comment.ID).SetStatus(biz.CommentStatusApproved).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Article.Update().Where(article.SlugEQ("post")).SetStatus(biz.ArticleStatusDraft).Save(ctx); err != nil {
		t.Fatal(err)
	}
	owned, err = mini.FindOwned(ctx, child.ID, actor.UserID, actor.AppID)
	if err != nil || owned.Visible() {
		t.Fatal("child visible under unpublished article", err)
	}
	if _, err = db.Article.Update().Where(article.SlugEQ("post")).SetStatus(biz.ArticleStatusPublished).Save(ctx); err != nil {
		t.Fatal(err)
	}
	oldHash := passed.InputHash
	input.Content = "答复 B"
	child, err = r.PutAuthorReply(ctx, root.Comment.ID, input, 1)
	if err != nil || child.ReplyVersion != 2 {
		t.Fatal("edit", err)
	}
	check, err := db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(child.ID)).Only(ctx)
	if err != nil || check.Generation <= passed.Generation || check.InputHash != "" || check.NameTraceID != "" || check.IdentityID != nil {
		t.Fatal("evidence not invalidated", err)
	}
	input.Content = "答复 A"
	child, err = r.PutAuthorReply(ctx, root.Comment.ID, input, 2)
	if err != nil || child.ReplyVersion != 3 {
		t.Fatal("ABA edit", err)
	}
	if biz.MiniCommentInputHash(child, actor.AppID, passed.IdentityID) == oldHash {
		t.Fatal("ABA hash reused")
	}
	if _, err = mini.CompleteCheck(ctx, passed, biz.MiniCommentVerdict{State: biz.MiniCommentPass, NameTraceID: "old", ContentTraceID: "old"}, false); err == nil {
		t.Fatal("stale callback accepted")
	}
	if _, err = r.PutAuthorReply(ctx, root.Comment.ID, input, 1); err != biz.ErrAuthorReplyConflict {
		t.Fatal("stale edit", err)
	}
	if _, err = r.PutAuthorReply(ctx, child.ID, input, 0); err != biz.ErrCommentInvalidArgument {
		t.Fatal("reply to reply", err)
	}
	if _, err = db.User.UpdateOneID(actor.UserID).SetRole(biz.UserRoleReader).Save(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = r.PutAuthorReply(ctx, root.Comment.ID, input, 3); err != biz.ErrAuthForbidden {
		t.Fatal("demoted author", err)
	}
	if err = repo.Delete(ctx, root.Comment.ID); err != nil {
		t.Fatal("delete thread", err)
	}
	if n, _ := db.Comment.Query().Where(comment.IDEQ(child.ID)).Count(ctx); n != 0 {
		t.Fatal("orphan reply")
	}
	if n, _ := db.WechatCommentCheck.Query().Where(wechatcommentcheck.CommentIDEQ(child.ID)).Count(ctx); n != 0 {
		t.Fatal("orphan evidence")
	}
}
func TestAuthorReplyMySQLConcurrentCreate(t *testing.T) {
	miniMySQL(t)
	ctx := context.Background()
	mini, db, actor := miniFixture(t)
	if _, e := db.User.UpdateOneID(actor.UserID).SetRole(biz.UserRoleAdmin).Save(ctx); e != nil {
		t.Fatal(e)
	}
	root := miniCreate(t, mini, actor, "root", "根评论")
	repo := NewCommentRepo(&Data{db: db})
	r := repo.(biz.AuthorReplyRepo)
	t.Cleanup(func() { _ = repo.Delete(ctx, root.Comment.ID) })
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := r.PutAuthorReply(ctx, root.Comment.ID, &biz.Comment{Content: "并发答复", DisplayName: "原始昵称", UserID: &actor.UserID, Status: biz.CommentStatusApproved}, 0)
			errs <- e
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	ok, conflicts := 0, 0
	for e := range errs {
		switch e {
		case nil:
			ok++
		case biz.ErrAuthorReplyConflict:
			conflicts++
		default:
			t.Fatal(e)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal("concurrent create did not serialize", ok, conflicts)
	}
	// Stale simultaneous editors must produce one winner without lost updates.
	errs = make(chan error, 2)
	start = make(chan struct{})
	for _, body := range []string{"修改 A", "修改 B"} {
		wg.Add(1)
		go func(content string) {
			defer wg.Done()
			<-start
			_, e := r.PutAuthorReply(ctx, root.Comment.ID, &biz.Comment{Content: content, DisplayName: "原始昵称", UserID: &actor.UserID, Status: biz.CommentStatusApproved}, 1)
			errs <- e
		}(body)
	}
	close(start)
	wg.Wait()
	close(errs)
	ok, conflicts = 0, 0
	for e := range errs {
		switch e {
		case nil:
			ok++
		case biz.ErrAuthorReplyConflict:
			conflicts++
		default:
			t.Fatal(e)
		}
	}
	if ok != 1 || conflicts != 1 {
		t.Fatal("concurrent edit lost update", ok, conflicts)
	}
	// Root deletion and a child edit serialize on the root; neither can orphan a reply.
	start = make(chan struct{})
	errs = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); <-start; errs <- repo.Delete(ctx, root.Comment.ID) }()
	go func() {
		defer wg.Done()
		<-start
		_, e := r.PutAuthorReply(ctx, root.Comment.ID, &biz.Comment{Content: "删除竞争", DisplayName: "原始昵称", UserID: &actor.UserID, Status: biz.CommentStatusApproved}, 2)
		errs <- e
	}()
	close(start)
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil && e != biz.ErrCommentNotFound {
			t.Fatal("delete race", e)
		}
	}
	if n, e := db.Comment.Query().Where(comment.Or(comment.IDEQ(root.Comment.ID), comment.ParentIDEQ(root.Comment.ID))).Count(ctx); e != nil || n != 0 {
		t.Fatal("delete race orphan", e)
	}

}

// Hold the thread to prove a first recheck owns its user lock before waiting
// for the parent. NOWAIT would succeed under the former inverted lock order.
func TestAuthorReplyMySQLRecheckLockOrder(t *testing.T) {
	_, sqlDB := miniMySQL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mini, db, actor := miniFixture(t)
	if _, e := db.User.UpdateOneID(actor.UserID).SetRole(biz.UserRoleAdmin).Save(ctx); e != nil {
		t.Fatal(e)
	}
	root := miniCreate(t, mini, actor, "lock-root", "锁序根评论")
	repo := NewCommentRepo(&Data{db: db})
	r := repo.(biz.AuthorReplyRepo)
	t.Cleanup(func() { _ = repo.Delete(context.Background(), root.Comment.ID) })
	child, e := r.PutAuthorReply(ctx, root.Comment.ID, &biz.Comment{Content: "尚未检测的答复", DisplayName: "原始昵称", UserID: &actor.UserID, Status: biz.CommentStatusApproved}, 0)
	if e != nil {
		t.Fatal(e)
	}
	hold, e := sqlDB.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = hold.Rollback() }()
	var id string
	if e = hold.QueryRowContext(ctx, "SELECT id FROM comments WHERE id=? FOR UPDATE", root.Comment.ID.String()).Scan(&id); e != nil {
		t.Fatal("hold parent failed")
	}
	rechecked := make(chan error, 1)
	go func() { _, _, err := mini.BeginRecheck(ctx, child.ID, actor, time.Minute); rechecked <- err }()
	// Wait until the recheck has acquired the author lock; each successful probe
	// immediately releases its own temporary lock before trying again.
	acquired := false
	for i := 0; i < 100; i++ {
		probe, err := sqlDB.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal("probe transaction failed")
		}
		err = probe.QueryRowContext(ctx, "SELECT id FROM users WHERE id=? FOR UPDATE NOWAIT", actor.UserID.String()).Scan(&id)
		_ = probe.Rollback()
		if err != nil {
			var me *gomysql.MySQLError
			if errors.As(err, &me) && me.Number == 3572 {
				acquired = true
				break
			}
			t.Fatal("unexpected lock probe error")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !acquired {
		t.Fatal("recheck did not acquire user before blocked parent")
	}
	edited := make(chan error, 1)
	go func() {
		_, err := r.PutAuthorReply(ctx, root.Comment.ID, &biz.Comment{Content: "同时编辑", DisplayName: "原始昵称", UserID: &actor.UserID, Status: biz.CommentStatusApproved}, 1)
		edited <- err
	}()
	if e = hold.Rollback(); e != nil {
		t.Fatal("release parent failed")
	}
	if e = <-rechecked; e != nil {
		t.Fatal("first recheck failed", e)
	}
	if e = <-edited; e != nil {
		t.Fatal("concurrent edit failed", e)
	}
}
