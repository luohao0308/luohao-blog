package data

import (
	"context"

	"github.com/luohao0308/luohao-blog/backend/internal/biz"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent"
	"github.com/luohao0308/luohao-blog/backend/internal/data/ent/about"
)

const aboutSingletonKey = "main"

type aboutRepo struct{ data *Data }

func NewAboutRepo(data *Data) biz.AboutRepo { return &aboutRepo{data: data} }

func toBizAbout(po *ent.About) *biz.About {
	if po == nil {
		return nil
	}
	return &biz.About{
		ID: po.ID, Name: po.Name, Headline: po.Headline, Intro: po.Intro, ProfileImage: po.ProfileImage,
		CurrentFocus: po.CurrentFocus, Skills: append([]biz.AboutSkillGroup(nil), po.Skills...),
		Timeline: append([]biz.AboutTimelineEntry(nil), po.Timeline...), Contacts: append([]biz.AboutContact(nil), po.Contacts...),
		RepresentativeLinks: append([]biz.AboutLink(nil), po.RepresentativeLinks...), UpdatedAt: po.UpdatedAt,
	}
}

func (r *aboutRepo) GetAbout(ctx context.Context) (*biz.About, error) {
	po, err := r.data.db.About.Query().Where(about.SingletonKeyEQ(aboutSingletonKey)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.ErrAboutNotFound
		}
		return nil, err
	}
	return toBizAbout(po), nil
}

func (r *aboutRepo) UpdateAbout(ctx context.Context, a *biz.About) (*biz.About, error) {
	current, err := r.data.db.About.Query().Where(about.SingletonKeyEQ(aboutSingletonKey)).Only(ctx)
	if ent.IsNotFound(err) {
		po, createErr := r.data.db.About.Create().SetSingletonKey(aboutSingletonKey).
			SetName(a.Name).SetHeadline(a.Headline).SetIntro(a.Intro).SetProfileImage(a.ProfileImage).
			SetCurrentFocus(a.CurrentFocus).SetSkills(a.Skills).SetTimeline(a.Timeline).SetContacts(a.Contacts).
			SetRepresentativeLinks(a.RepresentativeLinks).Save(ctx)
		if createErr != nil {
			return nil, createErr
		}
		return toBizAbout(po), nil
	}
	if err != nil {
		return nil, err
	}
	po, err := r.data.db.About.UpdateOneID(current.ID).
		SetName(a.Name).SetHeadline(a.Headline).SetIntro(a.Intro).SetProfileImage(a.ProfileImage).
		SetCurrentFocus(a.CurrentFocus).SetSkills(a.Skills).SetTimeline(a.Timeline).SetContacts(a.Contacts).
		SetRepresentativeLinks(a.RepresentativeLinks).Save(ctx)
	if err != nil {
		return nil, err
	}
	return toBizAbout(po), nil
}
