package services 

import (
	"sync"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var (
	urlStore = make(map[string]string)
	mu       sync.RWMutex
)

func StoreURL(shortCode string, originalURL string) {
	mu.Lock()
	defer mu.Unlock()

	urlStore[shortCode] = originalURL
}

func GetURL(shortCode string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()

	url, exists := urlStore[shortCode]
	return url, exists
}

func EncodeBase62(num int64) string {
	if num == 0 {
		return "0"
	}
	result := ""
	for num > 0 {
		index := num % 62
		result = string(alphabet[index]) + result
		num /= 62
	}

	return result
}