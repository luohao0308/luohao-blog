package service

import (
	"context"
	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OfficialAccountService struct {
	v1.UnimplementedOfficialAccountServiceServer
	uc *biz.WechatSyncUsecase
}

func NewOfficialAccountService(uc *biz.WechatSyncUsecase) *OfficialAccountService {
	return &OfficialAccountService{uc: uc}
}
func (s *OfficialAccountService) GetWechatSyncSettings(context.Context, *v1.GetWechatSyncSettingsRequest) (*v1.WechatSyncSettings, error) {
	c := s.uc.Settings()
	return &v1.WechatSyncSettings{Enabled: c.Enabled, Configured: c.Configured, Mode: c.Mode, AccountName: c.AccountName, AccountId: c.AccountID}, nil
}
func (s *OfficialAccountService) ListWechatSyncTasks(ctx context.Context, req *v1.ListWechatSyncTasksRequest) (*v1.WechatSyncTaskSet, error) {
	token, err := pagination.ParsePageToken(req)
	if err != nil {
		return nil, invalidListArgument(err)
	}
	if !pageOffsetWithinWindow(token.Offset) {
		return nil, invalidListArgument(errPageOffsetOutOfRange)
	}
	if req.PageSize <= 0 {
		req.PageSize = defaultPageSize
	}
	clampPageSize(&req.PageSize)
	tasks, err := s.uc.List(ctx, int(req.PageSize), int(token.Offset))
	if err != nil {
		return nil, err
	}
	out := &v1.WechatSyncTaskSet{Tasks: make([]*v1.WechatSyncTask, 0, len(tasks))}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, wechatTaskReply(t))
	}
	if len(tasks) >= int(req.PageSize) {
		out.NextPageToken = token.Next(req).String()
	}
	return out, nil
}
func (s *OfficialAccountService) RetryWechatSyncTask(ctx context.Context, req *v1.RetryWechatSyncTaskRequest) (*v1.WechatSyncTask, error) {
	t, err := s.uc.Retry(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return wechatTaskReply(t), nil
}
func (s *OfficialAccountService) SyncArticleToWechat(ctx context.Context, req *v1.SyncArticleToWechatRequest) (*v1.WechatSyncTask, error) {
	t, err := s.uc.SyncArticle(ctx, req.GetSlug())
	if err != nil {
		return nil, err
	}
	return wechatTaskReply(t), nil
}
func wechatTaskReply(t *biz.WechatSyncTask) *v1.WechatSyncTask {
	out := &v1.WechatSyncTask{Id: t.ID, Slug: t.Slug, Title: t.Article.Title, State: t.State, Mode: t.Mode, DraftId: t.DraftID, RemoteId: t.RemoteID, ArticleUrl: t.ArticleURL, LastError: t.LastError, Attempts: int32(t.Attempts), CreatedAt: timestamppb.New(t.CreatedAt), UpdatedAt: timestamppb.New(t.UpdatedAt), CanRetry: biz.WechatTaskCanRetry(t)}
	if !t.NextAttemptAt.IsZero() {
		out.NextAttemptAt = timestamppb.New(t.NextAttemptAt)
	}
	return out
}
