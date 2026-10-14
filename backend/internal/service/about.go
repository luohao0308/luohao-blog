package service

import (
	"context"

	v1 "github.com/luohao0308/luohao-blog/backend/api/blog/v1"
	"github.com/luohao0308/luohao-blog/backend/internal/biz"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type AboutService struct {
	v1.UnimplementedAboutServiceServer
	uc *biz.AboutUsecase
}

func NewAboutService(uc *biz.AboutUsecase) *AboutService { return &AboutService{uc: uc} }
func (s *AboutService) GetAbout(ctx context.Context, _ *v1.GetAboutRequest) (*v1.About, error) {
	a, err := s.uc.Get(ctx)
	if err != nil {
		return nil, err
	}
	return convertAboutReply(a), nil
}
func (s *AboutService) UpdateAbout(ctx context.Context, req *v1.UpdateAboutRequest) (*v1.About, error) {
	a, err := s.uc.Update(ctx, convertAbout(req.GetAbout()))
	if err != nil {
		return nil, err
	}
	return convertAboutReply(a), nil
}
func convertAbout(in *v1.About) *biz.About {
	if in == nil {
		return nil
	}
	a := &biz.About{Name: in.GetName(), Headline: in.GetHeadline(), Intro: in.GetIntro(), ProfileImage: in.GetProfileImage(), CurrentFocus: in.GetCurrentFocus()}
	for _, g := range in.GetSkills() {
		a.Skills = append(a.Skills, biz.AboutSkillGroup{Group: g.GetGroup(), Items: append([]string(nil), g.GetItems()...)})
	}
	for _, e := range in.GetTimeline() {
		a.Timeline = append(a.Timeline, biz.AboutTimelineEntry{Year: e.GetYear(), Text: e.GetText()})
	}
	for _, c := range in.GetContacts() {
		a.Contacts = append(a.Contacts, biz.AboutContact{Label: c.GetLabel(), Href: c.GetHref(), Text: c.GetText()})
	}
	for _, l := range in.GetRepresentativeLinks() {
		a.RepresentativeLinks = append(a.RepresentativeLinks, biz.AboutLink{Label: l.GetLabel(), Href: l.GetHref()})
	}
	return a
}
func convertAboutReply(in *biz.About) *v1.About {
	if in == nil {
		return nil
	}
	a := &v1.About{Id: in.ID.String(), Name: in.Name, Headline: in.Headline, Intro: in.Intro, ProfileImage: in.ProfileImage, CurrentFocus: in.CurrentFocus, UpdatedAt: timestamppb.New(in.UpdatedAt)}
	for _, g := range in.Skills {
		a.Skills = append(a.Skills, &v1.AboutSkillGroup{Group: g.Group, Items: g.Items})
	}
	for _, e := range in.Timeline {
		a.Timeline = append(a.Timeline, &v1.AboutTimelineEntry{Year: e.Year, Text: e.Text})
	}
	for _, c := range in.Contacts {
		a.Contacts = append(a.Contacts, &v1.AboutContact{Label: c.Label, Href: c.Href, Text: c.Text})
	}
	for _, l := range in.RepresentativeLinks {
		a.RepresentativeLinks = append(a.RepresentativeLinks, &v1.AboutLink{Label: l.Label, Href: l.Href})
	}
	return a
}
