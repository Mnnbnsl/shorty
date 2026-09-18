package cache

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

// Redirects is the shared cache instance, initialised by Init.
var Redirects *lru.Cache[string, string]

// Init creates the LRU cache with the given capacity.
// Call once at startup before serving requests.
// A capacity of 10_000 covers the vast majority of hot short codes
func Init(capacity int) {
	var err error
	Redirects, err = lru.New[string, string](capacity)
	if err != nil {
		panic("cache: invalid capacity: " + err.Error())
	}
}

func Get(shortCode string) (string, bool) {
	return Redirects.Get(shortCode)
}

func Set(shortCode, url string) {
	Redirects.Add(shortCode, url)
}
