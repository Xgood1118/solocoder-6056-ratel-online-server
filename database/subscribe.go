package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ratel-online/core/log"
	"github.com/ratel-online/server/bot"
)

type Subscription struct {
	RoomID        int64     `json:"roomId"`
	GroupID       int64     `json:"groupId"`
	Platform      string    `json:"platform"`
	NotifyEnabled bool      `json:"notifyEnabled"`
	CreatedAt     time.Time `json:"createdAt"`
}

type PushEventType struct {
	Type      string    `json:"type"`
	RoomID    int64     `json:"roomId"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

type PushBatch struct {
	Events        []PushEventType `json:"events"`
	LastTimestamp time.Time       `json:"lastTimestamp"`
	timer         *time.Timer
}

var (
	subscriptions []*Subscription
	pushBatches   = make(map[int64]*PushBatch)
	subMutex      sync.Mutex
	batchMutex    sync.Mutex
)

const subscriptionsFile = "./data/subscriptions.json"
const pushWindow = 5 * time.Second

var cardPattern = regexp.MustCompile(`\[([^\]]+)\]`)

func SaveSubscriptions() error {
	subMutex.Lock()
	defer subMutex.Unlock()
	dir := filepath.Dir(subscriptionsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(subscriptions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(subscriptionsFile, data, 0644)
}

func LoadSubscriptions() error {
	subMutex.Lock()
	defer subMutex.Unlock()
	if _, err := os.Stat(subscriptionsFile); os.IsNotExist(err) {
		subscriptions = make([]*Subscription, 0)
		return nil
	}
	data, err := os.ReadFile(subscriptionsFile)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &subscriptions); err != nil {
		return err
	}
	if subscriptions == nil {
		subscriptions = make([]*Subscription, 0)
	}
	log.Infof("[LoadSubscriptions] Loaded %d subscriptions\n", len(subscriptions))
	return nil
}

func SetRoomNotify(roomId int64, enabled bool) {
	subMutex.Lock()
	defer subMutex.Unlock()
	for _, sub := range subscriptions {
		if sub.RoomID == roomId {
			sub.NotifyEnabled = enabled
		}
	}
	_ = SaveSubscriptions()
}

func SubscribeGroup(roomId int64, groupId int64, platform string) {
	subMutex.Lock()
	defer subMutex.Unlock()
	for _, sub := range subscriptions {
		if sub.RoomID == roomId && sub.GroupID == groupId {
			sub.NotifyEnabled = true
			_ = SaveSubscriptions()
			return
		}
	}
	sub := &Subscription{
		RoomID:        roomId,
		GroupID:       groupId,
		Platform:      platform,
		NotifyEnabled: true,
		CreatedAt:     time.Now(),
	}
	subscriptions = append(subscriptions, sub)
	_ = SaveSubscriptions()
	log.Infof("[SubscribeGroup] Room %d subscribed by group %d (%s)\n", roomId, groupId, platform)
}

func UnsubscribeGroup(roomId int64, groupId int64) {
	subMutex.Lock()
	defer subMutex.Unlock()
	for i, sub := range subscriptions {
		if sub.RoomID == roomId && sub.GroupID == groupId {
			subscriptions = append(subscriptions[:i], subscriptions[i+1:]...)
			_ = SaveSubscriptions()
			log.Infof("[UnsubscribeGroup] Room %d unsubscribed by group %d\n", roomId, groupId)
			return
		}
	}
}

func formatCardContent(content string) string {
	return cardPattern.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[1 : len(match)-1]
		parts := strings.Fields(inner)
		keys := make([]int, 0, len(parts))
		for _, part := range parts {
			if key, err := strconv.Atoi(part); err == nil {
				keys = append(keys, key)
			}
		}
		if len(keys) > 0 {
			return PokerKeysToInline(keys)
		}
		return match
	})
}

func PushEvent(roomId int64, eventType string, content string) {
	batchMutex.Lock()
	defer batchMutex.Unlock()

	formattedContent := formatCardContent(content)

	event := PushEventType{
		Type:      eventType,
		RoomID:    roomId,
		Timestamp: time.Now(),
		Content:   formattedContent,
	}

	batch, exists := pushBatches[roomId]
	if !exists {
		batch = &PushBatch{
			Events: make([]PushEventType, 0),
		}
		pushBatches[roomId] = batch
	}

	batch.Events = append(batch.Events, event)
	batch.LastTimestamp = event.Timestamp

	if batch.timer != nil {
		batch.timer.Stop()
	}
	batch.timer = time.AfterFunc(pushWindow, func() {
		flushBatch(roomId)
	})
}

func flushBatch(roomId int64) {
	batchMutex.Lock()
	batch, exists := pushBatches[roomId]
	if !exists {
		batchMutex.Unlock()
		return
	}
	delete(pushBatches, roomId)
	batchMutex.Unlock()

	room := GetRoom(roomId)
	if room == nil || !room.NotifyEnabled {
		return
	}

	eventsByType := make(map[string][]string)
	for _, event := range batch.Events {
		eventsByType[event.Type] = append(eventsByType[event.Type], event.Content)
	}

	var messages []string
	for eventType, contents := range eventsByType {
		var header string
		switch eventType {
		case "result":
			header = "【对局结果】"
		case "skill":
			header = "【技能触发】"
		case "max_hand":
			header = "【最大牌型】"
		default:
			header = "【" + eventType + "】"
		}
		messages = append(messages, header+"\n"+strings.Join(contents, "\n"))
	}

	if len(messages) == 0 {
		return
	}

	fullMessage := strings.Join(messages, "\n\n")

	subMutex.Lock()
	var targetGroups []int64
	for _, sub := range subscriptions {
		if sub.RoomID == roomId && sub.NotifyEnabled {
			targetGroups = append(targetGroups, sub.GroupID)
		}
	}
	subMutex.Unlock()

	for _, groupId := range targetGroups {
		err := bot.SendGroupMessage(groupId, fullMessage)
		if err != nil {
			log.Errorf("[flushBatch] Failed to send message to group %d: %v\n", groupId, err)
		}
	}
}

func init() {
	err := LoadSubscriptions()
	if err != nil {
		log.Errorf("[subscribe.init] Failed to load subscriptions: %v\n", err)
	}
}
