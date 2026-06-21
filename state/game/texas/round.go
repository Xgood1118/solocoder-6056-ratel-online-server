package texas

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/ratel-online/core/model"
	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/bot"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

func nextRound(game *database.Texas) error {
	switch game.Round {
	case "start":
		return preFlopRound(game)
	case "per-flop":
		return flopRound(game)
	case "flop":
		return turnRound(game)
	case "turn":
		return riverRound(game)
	case "river":
		return settlementRound(game)
	default:
		return consts.ErrorsUnknownTexasRound
	}
}

func preFlopRound(game *database.Texas) error {
	game.Round = "per-flop"
	for id := range database.RoomPlayers(game.Room.ID) {
		player := database.GetPlayer(id)
		if player.Amount < 100 {
			player.Amount += 2000
			database.Broadcast(game.Room.ID, fmt.Sprintf("%s is too poor, system give him 2000\n", player.Name))
			bot.SendGroupMessage(bot.GroupID, fmt.Sprintf("%s is too poor, system give him 2000", player.Name))
		}
	}

	game.Pot += 30
	game.BBPlayer().Bet(20)
	game.SBPlayer().Bet(10)

	for id := range database.RoomPlayers(game.Room.ID) {
		player := database.GetPlayer(id)
		texasPlayer := game.Player(id)

		buf := bytes.Buffer{}
		buf.WriteString(fmt.Sprintf("Game starting!\n"))
		if game.SBPlayer().ID != player.ID {
			buf.WriteString(fmt.Sprintf("Your hand: %s\n", texasPlayer.Hand.TexasString()))
		}
		if game.BBPlayer().ID == player.ID {
			buf.WriteString("You are big blind, bet 20 automatically.\n")
		} else {
			buf.WriteString(fmt.Sprintf("Big blind: %s, Bet 20\n", game.Players[game.BB].Name))
		}
		if game.SBPlayer().ID == player.ID {
			buf.WriteString("You are small blind, bet 10 automatically.\n")
		} else {
			buf.WriteString(fmt.Sprintf("Small blind: %s, Bet 10\n", game.Players[game.SB].Name))
			buf.WriteString(fmt.Sprintf("Pre-flop round, please wait for small blind %s to bet\n", game.Players[game.SB].Name))
		}
		_ = player.WriteString(buf.String())
	}
	game.SBPlayer().State <- stateBet
	return nil
}

func flopRound(game *database.Texas) error {
	game.Round = "flop"
	game.MaxBetPlayer = nil
	game.Board = append(game.Board, game.Pool[1:4]...)
	game.Pool = game.Pool[4:]
	for _, c := range game.Board {
		game.ReplayCtx.BoardCards = append(game.ReplayCtx.BoardCards, c.Key)
	}
	addTexasReplayEvent(game, 0, database.ReplayEventMultiple, []int{1})
	database.Broadcast(game.Room.ID, fmt.Sprintf("Flop round, board: %s\n", game.Board.TexasString()))
	game.SBPlayer().State <- stateBet
	return nil
}

func turnRound(game *database.Texas) error {
	game.Round = "turn"
	game.MaxBetPlayer = nil
	game.Board = append(game.Board, game.Pool[1:2]...)
	game.Pool = game.Pool[2:]
	for _, c := range game.Board[len(game.Board)-1:] {
		game.ReplayCtx.BoardCards = append(game.ReplayCtx.BoardCards, c.Key)
	}
	addTexasReplayEvent(game, 0, database.ReplayEventMultiple, []int{2})
	database.Broadcast(game.Room.ID, fmt.Sprintf("Turn round, board: %s\n", game.Board.TexasString()))
	game.SBPlayer().State <- stateBet
	return nil
}

func riverRound(game *database.Texas) error {
	game.Round = "river"
	game.MaxBetPlayer = nil
	game.Board = append(game.Board, game.Pool[1:2]...)
	game.Pool = game.Pool[2:]
	for _, c := range game.Board[len(game.Board)-1:] {
		game.ReplayCtx.BoardCards = append(game.ReplayCtx.BoardCards, c.Key)
	}
	addTexasReplayEvent(game, 0, database.ReplayEventMultiple, []int{3})
	database.Broadcast(game.Room.ID, fmt.Sprintf("River round, board: %s\n", game.Board.TexasString()))
	game.SBPlayer().State <- stateBet
	return nil
}

func settlementRound(game *database.Texas) error {
	buf := bytes.Buffer{}
	buf.WriteString("Settlement round\n")
	buf.WriteString(fmt.Sprintf("Board: %s\n", game.Board.TexasString()))

	var maxFaces *model.TexasFaces
	var maxPlayers []int64
	var winnerIds []int64
	var maxHandKeys []int
	var maxHandType string
	var chipsWonPerPlayer map[int64]int = map[int64]int{}

	if game.Folded == len(game.Players)-1 {
		var winner *database.TexasPlayer
		for _, player := range game.Players {
			if !player.Folded {
				winner = player
				break
			}
		}
		if winner != nil {
			winner.Add(game.Pot)
			winnerIds = []int64{winner.ID}
			chipsWonPerPlayer[winner.ID] = int(game.Pot)
			buf.WriteString(fmt.Sprintf("Winner: %s, got all pot: %d\n", winner.Name, game.Pot))
		} else {
			buf.WriteString("All players folded\n")
		}
	} else {
		buf.WriteString("Players' hands:\n")
		for _, player := range game.Players {
			if player.Folded {
				continue
			}
			faces, err := poker.ParseTexasFaces(player.Hand, game.Board)
			if err != nil {
				return err
			}
			buf.WriteString(fmt.Sprintf("%s: %s, type: %s, score: %d\n", player.Name, player.Hand.TexasString(), faces.Type, faces.Score))
			if maxFaces == nil ||
				maxFaces.Type < faces.Type ||
				(maxFaces.Type == faces.Type && maxFaces.Score < faces.Score) {
				maxFaces = faces
				maxPlayers = []int64{player.ID}
				maxHandType = faces.Type.String()
				handKeys := make([]int, len(player.Hand))
				for i, c := range player.Hand {
					handKeys[i] = c.Key
				}
				maxHandKeys = handKeys
				continue
			}
			if maxFaces.Type == faces.Type && maxFaces.Score == faces.Score {
				maxPlayers = append(maxPlayers, player.ID)
			}
		}
		winners := make([]*database.TexasPlayer, 0)
		for _, id := range maxPlayers {
			winners = append(winners, game.Player(id))
		}
		winnerIds = maxPlayers
		eachWin := int(game.Pot) / len(winners)
		for _, winner := range winners {
			winner.Add(uint(eachWin))
			chipsWonPerPlayer[winner.ID] = eachWin
		}
		if len(winners) == 1 {
			buf.WriteString(fmt.Sprintf("Winner: %s, got all pot: %d\n", winners[0].Name, game.Pot))
		} else {
			buf.WriteString("Winners: ")
			for i, winner := range winners {
				if i != 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(winner.Name)
			}
			buf.WriteString(fmt.Sprintf(", half all pot: %d\n", game.Pot))
		}
	}
	buf.WriteString(fmt.Sprintf("Please room owner %s to start a new game\n", database.GetPlayer(game.Room.Creator).Name))
	database.Broadcast(game.Room.ID, buf.String())

	game.ReplayCtx.EndTime = time.Now()
	game.ReplayCtx.Winners = winnerIds
	game.ReplayCtx.MaxMultiple = int(game.Pot)

	for _, pid := range game.ReplayCtx.PlayerHands {
		_ = pid
	}
	for _, tp := range game.Players {
		won := false
		for _, wid := range winnerIds {
			if tp.ID == wid {
				won = true
				break
			}
		}
		database.UpdateProfileStats(tp.ID, won, game.ReplayCtx.MaxMultiple, false)

		chipsWon := chipsWonPerPlayer[tp.ID]
		ctx := map[string]interface{}{
			"won":       won,
			"isTexas":   true,
			"chipsWon":  chipsWon,
			"handType":  maxHandType,
			"handCards": len(tp.Hand),
		}

		profile := database.GetProfile(tp.ID)
		newBadges := database.CheckAchievements(profile, ctx)
		for _, badge := range newBadges {
			database.Broadcast(game.Room.ID, fmt.Sprintf("🎉 %s 解锁成就【%s】- %s\n", database.GetPlayer(tp.ID).Name, badge.Name, badge.Description))
		}
	}

	_ = database.SaveReplay(game.ReplayCtx)

	if game.Room.NotifyEnabled {
		winnerNames := make([]string, 0)
		for _, wid := range winnerIds {
			winnerNames = append(winnerNames, database.GetPlayer(wid).Name)
		}
		resultContent := fmt.Sprintf("房间 %d - 胜者: %s, 奖池: %d", game.Room.ID, strings.Join(winnerNames, ","), game.Pot)
		database.PushEvent(game.Room.ID, "result", resultContent)

		if len(maxHandKeys) > 0 {
			handContent := fmt.Sprintf("最大牌型 [%s]: %s", maxHandType, database.PokerKeysToInline(maxHandKeys))
			database.PushEvent(game.Room.ID, "max_hand", handContent)
		}
	}

	room := game.Room
	room.State = consts.RoomStateWaiting
	for _, player := range game.Players {
		player.State <- stateWaiting
	}
	return nil
}
