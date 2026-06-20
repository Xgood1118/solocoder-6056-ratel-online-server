package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type ReplayEventType string

const (
	ReplayEventPlay     ReplayEventType = "play"
	ReplayEventSkill    ReplayEventType = "skill"
	ReplayEventMultiple ReplayEventType = "multiple"
	ReplayEventChat     ReplayEventType = "chat"
)

type ReplayEvent struct {
	Type      ReplayEventType `json:"type"`
	Timestamp int64           `json:"timestamp"`
	PlayerID  int64           `json:"playerId"`
	Data      []int           `json:"data"`
	DelayMs   int64           `json:"delayMs"`
}

type ReplayComment struct {
	PlayerID  int64     `json:"playerId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type ReplayRecord struct {
	ID          int64                 `json:"id"`
	RoomID      int64                 `json:"roomId"`
	GameType    int                   `json:"gameType"`
	StartTime   time.Time             `json:"startTime"`
	EndTime     time.Time             `json:"endTime"`
	Events      []ReplayEvent         `json:"events"`
	BoardCards  []int                 `json:"boardCards"`
	PlayerHands map[int64][]int       `json:"playerHands"`
	Winners     []int64               `json:"winners"`
	MaxMultiple int                   `json:"maxMultiple"`
	Likes       int                   `json:"likes"`
	Comments    []ReplayComment       `json:"comments"`
	LikePlayers map[int64]bool        `json:"-"`
}

var replayId int64 = 0
var replayMutex sync.Mutex
var replays = map[int64]*ReplayRecord{}
var roomReplays = map[int64][]*ReplayRecord{}

func SaveReplay(record *ReplayRecord) error {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	record.ID = atomic.AddInt64(&replayId, 1)
	record.LikePlayers = map[int64]bool{}

	replays[record.ID] = record
	roomReplays[record.RoomID] = append(roomReplays[record.RoomID], record)

	dir := filepath.Join(".", "data", "replays", strconv.FormatInt(record.RoomID, 10))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(dir, strconv.FormatInt(record.ID, 10)+".json")
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func GetRoomReplays(roomId int64, limit int) []*ReplayRecord {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	list, ok := roomReplays[roomId]
	if !ok {
		return []*ReplayRecord{}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID > list[j].ID
	})

	if limit > 0 && limit < len(list) {
		return list[:limit]
	}
	return list
}

func GetReplay(replayId int64) *ReplayRecord {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	if record, ok := replays[replayId]; ok {
		return record
	}
	return nil
}

func AddReplayLike(replayId int64, playerId int64) {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	record, ok := replays[replayId]
	if !ok {
		return
	}

	if record.LikePlayers == nil {
		record.LikePlayers = map[int64]bool{}
	}

	if !record.LikePlayers[playerId] {
		record.LikePlayers[playerId] = true
		record.Likes++
	}
}

func AddReplayComment(replayId int64, playerId int64, content string) {
	replayMutex.Lock()
	defer replayMutex.Unlock()

	record, ok := replays[replayId]
	if !ok {
		return
	}

	if len(content) > 20 {
		content = content[:20]
	}

	record.Comments = append(record.Comments, ReplayComment{
		PlayerID:  playerId,
		Content:   content,
		CreatedAt: time.Now(),
	})
}
