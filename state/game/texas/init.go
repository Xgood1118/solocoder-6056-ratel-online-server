package texas

import (
	"time"

	"github.com/ratel-online/core/util/poker"
	"github.com/ratel-online/server/consts"
	"github.com/ratel-online/server/database"
)

func addTexasReplayEvent(game *database.Texas, playerId int64, eventType database.ReplayEventType, data []int) {
	if game.ReplayCtx == nil {
		return
	}
	now := time.Now().UnixMilli()
	delayMs := int64(0)
	if game.LastEventTs > 0 {
		delayMs = now - game.LastEventTs
	}
	game.LastEventTs = now
	game.ReplayCtx.Events = append(game.ReplayCtx.Events, database.ReplayEvent{
		Type:      eventType,
		Timestamp: now,
		PlayerID:  playerId,
		Data:      data,
		DelayMs:   delayMs,
	})
}

func Init(room *database.Room) (game database.RoomGame, err error) {
	if room.Game != nil {
		return resetGame(room)
	}
	return createGame(room)
}

func createGame(room *database.Room) (database.RoomGame, error) {
	base := poker.GetTexasBase()
	base.Shuffle(len(base), 1)

	index := 0
	roomPlayers := database.RoomPlayers(room.ID)
	players := make([]*database.TexasPlayer, 0)
	playerHands := map[int64][]int{}
	for playerId := range roomPlayers {
		player := database.GetPlayer(playerId)
		hand := base[index*2 : (index+1)*2]
		handKeys := make([]int, len(hand))
		for i, c := range hand {
			handKeys[i] = c.Key
		}
		playerHands[playerId] = handKeys
		players = append(players, &database.TexasPlayer{
			ID:    playerId,
			Name:  player.Name,
			State: make(chan int, 1),
			Hand:  hand,
		})
		index++
	}
	game := &database.Texas{
		Room:         room,
		Players:      players,
		Pot:          0,
		BB:           0,
		SB:           1,
		Pool:         base[len(players)*2:],
		MaxBetAmount: 20,
		Round:        "start",
		MaxHandType:  "",
		MaxHandScore: 0,
		ReplayCtx: &database.ReplayRecord{
			RoomID:      room.ID,
			GameType:    consts.GameTypeTexas,
			StartTime:   time.Now(),
			Events:      []database.ReplayEvent{},
			BoardCards:  []int{},
			PlayerHands: playerHands,
			Winners:     []int64{},
			MaxMultiple: 0,
			Likes:       0,
			Comments:    []database.ReplayComment{},
		},
		LastEventTs: time.Now().UnixMilli(),
	}
	return game, nextRound(game)
}

func resetGame(room *database.Room) (database.RoomGame, error) {
	base := poker.GetTexasBase()
	base.Shuffle(len(base), 1)
	game := room.Game.(*database.Texas)

	texasPlayers := make(map[int64]*database.TexasPlayer)
	for _, texasPlayer := range game.Players {
		texasPlayers[texasPlayer.ID] = texasPlayer
	}

	index := 0
	roomPlayers := database.RoomPlayers(room.ID)
	players := make([]*database.TexasPlayer, 0)
	playerHands := map[int64][]int{}
	for playerId := range roomPlayers {
		hand := base[index*2 : (index+1)*2]
		handKeys := make([]int, len(hand))
		for i, c := range hand {
			handKeys[i] = c.Key
		}
		playerHands[playerId] = handKeys
		if texasPlayer, ok := texasPlayers[playerId]; ok {
			texasPlayer.Reset()
			texasPlayer.Hand = hand
			players = append(players, texasPlayer)
		} else {
			player := database.GetPlayer(playerId)
			players = append(players, &database.TexasPlayer{
				ID:    playerId,
				Name:  player.Name,
				State: make(chan int, 1),
				Hand:  hand,
			})
		}
		index++
	}
	newGame := &database.Texas{
		Room:         room,
		Players:      players,
		Pot:          0,
		BB:           (game.BB + 1) % len(players),
		SB:           (game.BB + 2) % len(players),
		Pool:         base[len(players)*2:],
		MaxBetAmount: 20,
		Round:        "start",
		MaxHandType:  "",
		MaxHandScore: 0,
		ReplayCtx: &database.ReplayRecord{
			RoomID:      room.ID,
			GameType:    consts.GameTypeTexas,
			StartTime:   time.Now(),
			Events:      []database.ReplayEvent{},
			BoardCards:  []int{},
			PlayerHands: playerHands,
			Winners:     []int64{},
			MaxMultiple: 0,
			Likes:       0,
			Comments:    []database.ReplayComment{},
		},
		LastEventTs: time.Now().UnixMilli(),
	}
	return newGame, nextRound(newGame)
}

func nextPlayer(current *database.Player, game *database.Texas, state int) error {
	next := game.NextPlayer(current.ID)
	if next != nil {
		next.State <- state
	}
	return nil
}
