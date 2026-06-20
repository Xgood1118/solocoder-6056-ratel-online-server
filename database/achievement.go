package database

import (
	"sort"
	"sync"
	"time"
)

var achievementMu sync.Mutex

type Badge struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UnlockedAt  time.Time `json:"unlockedAt"`
}

type AchievementDef struct {
	ID          string
	Name        string
	Description string
	Condition   func(profile *PlayerProfile, gameContext map[string]interface{}) bool
}

var achievementDefs = []*AchievementDef{
	{
		ID:          "one_pillar_sky",
		Name:        "一柱擎天",
		Description: "单局最大倍数达到1024倍",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			return profile.MaxLaiZiMultiple >= 1024
		},
	},
	{
		ID:          "last_stand",
		Name:        "绝处逢生",
		Description: "手牌只剩1张时获胜",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			if gameContext == nil {
				return false
			}
			won, ok := gameContext["won"].(bool)
			if !ok || !won {
				return false
			}
			handCards, ok := gameContext["handCards"].(int)
			return ok && handCards == 1
		},
	},
	{
		ID:          "laizi_king",
		Name:        "癞子王",
		Description: "癞子模式累计获胜50局",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			return profile.LaiZiWins >= 50
		},
	},
	{
		ID:          "skill_master",
		Name:        "技能大师",
		Description: "累计使用技能100次",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			total := 0
			for _, count := range profile.SkillUses {
				total += count
			}
			return total >= 100
		},
	},
	{
		ID:          "ever_victorious",
		Name:        "常胜将军",
		Description: "累计获胜100局",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			return profile.Wins >= 100
		},
	},
	{
		ID:          "hundred_battles",
		Name:        "百战老兵",
		Description: "累计游戏500局",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			return profile.TotalGames >= 500
		},
	},
	{
		ID:          "newbie",
		Name:        "初出茅庐",
		Description: "完成第一局游戏",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			return profile.TotalGames >= 1
		},
	},
	{
		ID:          "god_of_gambling",
		Name:        "赌神",
		Description: "德州扑克单局赢取10000筹码",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			if gameContext == nil {
				return false
			}
			isTexas, ok := gameContext["isTexas"].(bool)
			if !ok || !isTexas {
				return false
			}
			chipsWon, ok := gameContext["chipsWon"].(int)
			return ok && chipsWon >= 10000
		},
	},
	{
		ID:          "straight_flush",
		Name:        "同花顺",
		Description: "德州扑克中打出同花顺",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			if gameContext == nil {
				return false
			}
			isTexas, ok := gameContext["isTexas"].(bool)
			if !ok || !isTexas {
				return false
			}
			handType, ok := gameContext["handType"].(string)
			return ok && handType == "straight_flush"
		},
	},
	{
		ID:          "bomb_expert",
		Name:        "炸弹专家",
		Description: "斗地主单局打出3个炸弹",
		Condition: func(profile *PlayerProfile, gameContext map[string]interface{}) bool {
			if gameContext == nil {
				return false
			}
			isLandlord, ok := gameContext["isLandlord"].(bool)
			if !ok || !isLandlord {
				return false
			}
			bombCount, ok := gameContext["bombCount"].(int)
			return ok && bombCount >= 3
		},
	},
}

func CheckAchievements(profile *PlayerProfile, gameContext map[string]interface{}) []Badge {
	achievementMu.Lock()
	defer achievementMu.Unlock()

	newBadges := []Badge{}
	unlockedBadgeIDs := map[string]bool{}
	for _, badge := range profile.Badges {
		unlockedBadgeIDs[badge.ID] = true
	}

	for _, def := range achievementDefs {
		if unlockedBadgeIDs[def.ID] {
			continue
		}
		if def.Condition(profile, gameContext) {
			badge := UnlockBadge(profile, def.ID)
			if badge != nil {
				newBadges = append(newBadges, *badge)
			}
		}
	}

	sort.Slice(profile.Badges, func(i, j int) bool {
		return profile.Badges[i].UnlockedAt.After(profile.Badges[j].UnlockedAt)
	})

	if len(newBadges) > 0 {
		SaveProfile(profile)
	}

	return newBadges
}

func UnlockBadge(profile *PlayerProfile, badgeId string) *Badge {
	for _, badge := range profile.Badges {
		if badge.ID == badgeId {
			return &badge
		}
	}

	for _, def := range achievementDefs {
		if def.ID == badgeId {
			badge := Badge{
				ID:          def.ID,
				Name:        def.Name,
				Description: def.Description,
				UnlockedAt:  time.Now(),
			}
			profile.Badges = append(profile.Badges, badge)
			return &badge
		}
	}
	return nil
}
