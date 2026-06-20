package database

import (
	"strconv"
	"strings"
)

func CardKeyToInline(key int) string {
	switch key {
	case 1:
		return "a"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	case 6:
		return "6"
	case 7:
		return "7"
	case 8:
		return "8"
	case 9:
		return "9"
	case 10:
		return "j"
	case 11:
		return "q"
	case 12:
		return "k"
	case 13:
		return "a"
	case 14:
		return "s"
	case 15:
		return "x"
	default:
		return strconv.Itoa(key)
	}
}

func PokerKeysToInline(keys []int) string {
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = CardKeyToInline(key)
	}
	return strings.Join(parts, " ")
}
