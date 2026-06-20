package state

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

type home struct{}

func (*home) Next(player *database.Player) (consts.StateID, error) {
	buf := bytes.Buffer{}
	buf.WriteString("1.Join\n")
	buf.WriteString("2.New\n")
	buf.WriteString("profile. 查看个人档案\n")
	err := player.WriteString(buf.String())
	if err != nil {
		return 0, player.WriteError(err)
	}
	player.StartTransaction()
	defer player.StopTransaction()
	for {
		input, err := player.AskForString()
		if err != nil {
			return 0, player.WriteError(err)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "profile" {
			err := showProfile(player)
			if err != nil {
				_ = player.WriteError(err)
			}
			return 0, nil
		}
		selected := 0
		if input == "1" || input == "join" {
			selected = 1
		} else if input == "2" || input == "new" {
			selected = 2
		}
		if selected == 1 {
			return consts.StateJoin, nil
		} else if selected == 2 {
			return consts.StateCreate, nil
		}
		_ = player.WriteError(consts.ErrorsInputInvalid)
	}
}

func showProfile(player *database.Player) error {
	profile := database.GetProfile(player.ID)
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("===== %s 的个人档案 =====\n", player.Name))
	buf.WriteString(fmt.Sprintf("累计游戏局数: %d\n", profile.TotalGames))
	buf.WriteString(fmt.Sprintf("累计胜利局数: %d\n", profile.Wins))
	buf.WriteString(fmt.Sprintf("胜率: %.1f%%\n", float64(profile.Wins)/float64(max(1, profile.TotalGames))*100))
	buf.WriteString(fmt.Sprintf("癞子模式最大倍数: %d倍\n", profile.MaxLaiZiMultiple))

	totalSkillUses := 0
	for _, count := range profile.SkillUses {
		totalSkillUses += count
	}
	buf.WriteString(fmt.Sprintf("累计技能使用次数: %d\n", totalSkillUses))
	buf.WriteString("\n")

	if len(profile.Badges) == 0 {
		buf.WriteString("尚未解锁任何成就徽章\n")
	} else {
		buf.WriteString(fmt.Sprintf("===== 徽章墙 (%d枚) =====\n", len(profile.Badges)))
		for i, badge := range profile.Badges {
			buf.WriteString(fmt.Sprintf("%d. 【%s】- %s\n   解锁时间: %s\n",
				i+1, badge.Name, badge.Description,
				badge.UnlockedAt.Format("2006-01-02 15:04")))
		}
	}
	buf.WriteString("========================\n")
	return player.WriteString(buf.String())
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (*home) Exit(player *database.Player) consts.StateID {
	return 0
}
