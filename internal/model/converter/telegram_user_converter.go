package converter

import (
	json "github.com/bytedance/sonic"
	"sort"

	"github.com/Fadlihardiyanto/telegram-management-app/internal/entity"
	"github.com/Fadlihardiyanto/telegram-management-app/internal/model"
)

// TelegramUserToResponse converts TelegramUser entity to TelegramUserResponse model
func TelegramUserToResponse(user *entity.TelegramUser) *model.TelegramUserResponse {
	if user == nil {
		return nil
	}

	return &model.TelegramUserResponse{
		ID:             user.ID,
		TelegramUserID: user.TelegramUserID,
		Username:       user.Username,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		CreatedAt:      user.CreatedAt,
	}
}

// TelegramUsersToResponse converts multiple TelegramUser entities to TelegramUserResponse models
func TelegramUsersToResponse(users []entity.TelegramUser) []model.TelegramUserResponse {
	if len(users) == 0 {
		return []model.TelegramUserResponse{}
	}

	responses := make([]model.TelegramUserResponse, len(users))
	for i, user := range users {
		responses[i] = *TelegramUserToResponse(&user)
	}
	return responses
}

// ── Member converters (telegram_users JOIN subscriptions) ───────────

// subscriptionToBrief converts a Subscription entity (with preloaded Package) to MemberSubscriptionBrief.
func subscriptionToBrief(sub entity.Subscription) model.MemberSubscriptionBrief {
	return model.MemberSubscriptionBrief{
		ID:          sub.ID,
		PackageID:   sub.PackageID,
		PackageName: sub.Package.Name,
		Status:      sub.Status,
		ActivatedAt: sub.ActivatedAt,
		ExpiredAt:   sub.ExpiredAt,
		AutoRenew:   sub.AutoRenew,
		KickedAt:    sub.KickedAt,
	}
}

// pickPrimarySubscription returns the most relevant subscription:
// active first (latest expired_at), then latest expired.
func pickPrimarySubscription(subs []entity.Subscription) *model.MemberSubscriptionBrief {
	if len(subs) == 0 {
		return nil
	}

	var active []entity.Subscription
	for _, s := range subs {
		if s.Status == "active" {
			active = append(active, s)
		}
	}

	pick := subs[0]
	if len(active) > 0 {
		pick = active[0]
		for _, s := range active[1:] {
			if s.ExpiredAt.After(pick.ExpiredAt) {
				pick = s
			}
		}
	} else {
		// No active → pick latest expired
		for _, s := range subs[1:] {
			if s.ExpiredAt.After(pick.ExpiredAt) {
				pick = s
			}
		}
	}

	brief := subscriptionToBrief(pick)
	return &brief
}

// MembersToResponse converts a slice of AggregatedMemberRow to MemberResponse slice.
// orderCounts maps user.ID → total orders for that user.
func MembersToResponse(users []model.AggregatedMemberRow, orderCounts map[string]int64) []model.MemberResponse {
	if len(users) == 0 {
		return []model.MemberResponse{}
	}

	responses := make([]model.MemberResponse, 0, len(users))
	for i := range users {
		count := orderCounts[users[i].ID.String()]
		
		var subscriptions []model.MemberSubscriptionBrief
		if len(users[i].Subscriptions) > 0 && string(users[i].Subscriptions) != "null" {
			_ = json.Unmarshal(users[i].Subscriptions, &subscriptions)
		}
		if subscriptions == nil {
			subscriptions = []model.MemberSubscriptionBrief{}
		}

		res := model.MemberResponse{
			ID:             users[i].ID,
			TelegramUserID: users[i].TelegramUserID,
			Username:       users[i].Username,
			FirstName:      users[i].FirstName,
			LastName:       users[i].LastName,
			Phone:          users[i].Phone,
			GlobalStatus:   users[i].GlobalStatus,
			Subscriptions:  subscriptions,
			TotalOrders:    count,
			CreatedAt:      users[i].CreatedAt,
		}
		responses = append(responses, res)
	}
	return responses
}

// MemberDetailToResponse converts a TelegramUser (with all subscriptions preloaded) to MemberDetailResponse.
func MemberDetailToResponse(user *entity.TelegramUser, totalOrders int64) *model.MemberDetailResponse {
	if user == nil {
		return nil
	}

	subs := make([]model.MemberSubscriptionBrief, 0, len(user.Subscriptions))
	for _, s := range user.Subscriptions {
		subs = append(subs, subscriptionToBrief(s))
	}

	// Sort: active first, then by expired_at desc
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].Status == "active" && subs[j].Status != "active" {
			return true
		}
		if subs[i].Status != "active" && subs[j].Status == "active" {
			return false
		}
		return subs[i].ExpiredAt.After(subs[j].ExpiredAt)
	})

	return &model.MemberDetailResponse{
		ID:             user.ID,
		TelegramUserID: user.TelegramUserID,
		Username:       user.Username,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Phone:          user.Phone,
		Subscriptions:  subs,
		TotalOrders:    totalOrders,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}
}
