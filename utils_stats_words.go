package main

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/neonxp/StemmerRu"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const statsWordCacheSize = 4096
const statsWordCacheMaxKeyBytes = 128

type statsWordCache struct {
	mu    sync.Mutex
	ids   map[string]int64
	order []string
	next  int
	loads singleflight.Group
}

var statsWordCaches = struct {
	sync.Mutex
	pool  *sql.DB
	cache *statsWordCache
}{}

// Bind IDs to the actual database, not a GORM session: a different connection
// pool must never reuse another database's dictionary IDs.
func statsWordsFor(db *gorm.DB) (*statsWordCache, error) {
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	statsWordCaches.Lock()
	defer statsWordCaches.Unlock()
	if statsWordCaches.pool != pool {
		statsWordCaches.pool = pool
		statsWordCaches.cache = &statsWordCache{ids: make(map[string]int64)}
	}
	return statsWordCaches.cache, nil
}

func (c *statsWordCache) get(stem string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.ids[stem]
	return id, ok
}

func (c *statsWordCache) put(stem string, id int64) {
	if id <= 0 || len(stem) > statsWordCacheMaxKeyBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.ids[stem]; exists {
		return
	}
	if len(c.order) < statsWordCacheSize {
		c.order = append(c.order, stem)
	} else {
		delete(c.ids, c.order[c.next])
		c.order[c.next] = stem
		c.next = (c.next + 1) % statsWordCacheSize
	}
	c.ids[stem] = id
}

func lookupStatsWord(word string) (int64, error) {
	cache, err := statsWordsFor(&DB)
	if err != nil {
		return 0, err
	}
	stem := StemmerRu.Stem(word)
	if id, ok := cache.get(stem); ok {
		return id, nil
	}
	// Concurrent requests for one uncached stem share its lookup/create. The
	// cache mutex is never held across database calls, so other words proceed.
	value, err, _ := cache.loads.Do(stem, func() (any, error) {
		if id, ok := cache.get(stem); ok {
			return id, nil
		}
		var row StatsWords
		result := DB.Model(&StatsWords{}).Select("id").Where("short_word = ?", stem).Find(&row)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			row.Word, row.ShortWord = word, stem
			if err := DB.Create(&row).Error; err != nil {
				return nil, err
			}
		}
		if row.ID <= 0 {
			return nil, fmt.Errorf("word lookup returned an invalid ID")
		}
		cache.put(stem, row.ID)
		return row.ID, nil
	})
	if err != nil {
		return 0, err
	}
	return value.(int64), nil
}
