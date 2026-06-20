package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/core/util/json"
)

var profileMu sync.Mutex
var profileCache = map[int64]*PlayerProfile{}

type PlayerProfile struct {
	PlayerID         int64           `json:"playerId"`
	TotalGames       int             `json:"totalGames"`
	Wins             int             `json:"wins"`
	LaiZiWins        int             `json:"laiZiWins"`
	MaxLaiZiMultiple int             `json:"maxLaiZiMultiple"`
	SkillUses        map[int]int     `json:"skillUses"`
	Badges           []Badge         `json:"badges"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

func GetProfile(playerId int64) *PlayerProfile {
	profileMu.Lock()
	defer profileMu.Unlock()

	if profile, ok := profileCache[playerId]; ok {
		return profile
	}

	profile := loadProfile(playerId)
	if profile == nil {
		profile = &PlayerProfile{
			PlayerID:         playerId,
			TotalGames:       0,
			Wins:             0,
			LaiZiWins:        0,
			MaxLaiZiMultiple: 0,
			SkillUses:        map[int]int{},
			Badges:           []Badge{},
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		saveProfile(profile)
	}

	profileCache[playerId] = profile
	return profile
}

func UpdateProfileStats(playerId int64, won bool, maxMultiple int, isLaiZi bool) {
	profile := GetProfile(playerId)
	profileMu.Lock()
	defer profileMu.Unlock()

	profile.TotalGames++
	if won {
		profile.Wins++
		if isLaiZi {
			profile.LaiZiWins++
		}
	}
	if maxMultiple > profile.MaxLaiZiMultiple {
		profile.MaxLaiZiMultiple = maxMultiple
	}
	profile.UpdatedAt = time.Now()
	saveProfile(profile)
}

func IncrementSkillUse(playerId int64, skillId int) {
	profile := GetProfile(playerId)
	profileMu.Lock()
	defer profileMu.Unlock()

	profile.SkillUses[skillId]++
	profile.UpdatedAt = time.Now()
	saveProfile(profile)
}

func SaveProfile(profile *PlayerProfile) {
	profileMu.Lock()
	defer profileMu.Unlock()

	profile.UpdatedAt = time.Now()
	saveProfile(profile)
}

func loadProfile(playerId int64) *PlayerProfile {
	path := fmt.Sprintf("./data/profiles/%d.json", playerId)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var profile PlayerProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		log.Error(err)
		return nil
	}
	return &profile
}

func saveProfile(profile *PlayerProfile) {
	dir := "./data/profiles"
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Error(err)
		return
	}

	path := filepath.Join(dir, fmt.Sprintf("%d.json", profile.PlayerID))
	data := json.Marshal(profile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Error(err)
	}
}
