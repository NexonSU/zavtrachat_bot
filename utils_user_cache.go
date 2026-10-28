package main

import (
	"database/sql"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const userCacheSize = 4096
const userCacheTTL = 5 * time.Minute
const userCacheMaxBytes = 1024

type savedUser struct {
	user gotgbot.User
	at   time.Time
}

type userWriteCache struct {
	mu    sync.Mutex
	users map[int64]savedUser
	order []int64
	next  int
	// Serialize writes for one user without holding the cache mutex over SQL.
	locks [64]sync.Mutex
}

var userWriteCaches = struct {
	sync.Mutex
	pool  *sql.DB
	cache *userWriteCache
}{}

func userWritesFor(db *gorm.DB) (*userWriteCache, error) {
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	userWriteCaches.Lock()
	defer userWriteCaches.Unlock()
	if userWriteCaches.pool != pool {
		userWriteCaches.pool = pool
		userWriteCaches.cache = &userWriteCache{users: make(map[int64]savedUser)}
	}
	return userWriteCaches.cache, nil
}

func saveUser(db *gorm.DB, user gotgbot.User) error {
	cache, err := userWritesFor(db)
	if err != nil {
		return err
	}
	return cache.save(db, user, time.Time{})
}

func (c *userWriteCache) save(db *gorm.DB, user gotgbot.User, now time.Time) error {
	lock := &c.locks[uint64(user.Id)%uint64(len(c.locks))]
	lock.Lock()
	defer lock.Unlock()
	// Start the TTL after acquiring the user's lock. A waiting goroutine may
	// have started earlier than the goroutine that just populated the cache.
	if now.IsZero() {
		now = time.Now()
	}
	c.mu.Lock()
	previous, ok := c.users[user.Id]
	c.mu.Unlock()
	if age := now.Sub(previous.at); ok && previous.user == user && age >= 0 && age < userCacheTTL {
		return nil
	}
	// On a cache miss/expiry, still avoid a physical UPDATE when the database
	// already has the same fields. Derive columns from the model so future
	// Telegram User fields cannot silently stop being saved.
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(&user); err != nil {
		return err
	}
	var changed []clause.Expression
	for _, field := range stmt.Schema.Fields {
		if field.DBName != "" && !field.PrimaryKey {
			changed = append(changed, clause.Expr{SQL: "? IS DISTINCT FROM ?", Vars: []any{
				clause.Column{Table: clause.CurrentTable, Name: field.DBName},
				clause.Column{Table: "excluded", Name: field.DBName},
			}})
		}
	}
	err := db.Session(&gorm.Session{SkipDefaultTransaction: true}).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}}, UpdateAll: true,
		Where: clause.Where{Exprs: []clause.Expression{clause.Or(changed...)}},
	}).Create(&user).Error
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		// Retry after any error rather than trusting an earlier cached snapshot.
		if cached, ok := c.users[user.Id]; ok {
			cached.at = time.Time{}
			c.users[user.Id] = cached
		}
		return err
	}
	if user.Id <= 0 || len(user.FirstName)+len(user.LastName)+len(user.Username)+len(user.LanguageCode) > userCacheMaxBytes {
		if cached, ok := c.users[user.Id]; ok {
			cached.at = time.Time{}
			c.users[user.Id] = cached
		}
		return nil
	}
	if _, exists := c.users[user.Id]; !exists {
		if len(c.order) < userCacheSize {
			c.order = append(c.order, user.Id)
		} else {
			delete(c.users, c.order[c.next])
			c.order[c.next] = user.Id
			c.next = (c.next + 1) % userCacheSize
		}
	}
	c.users[user.Id] = savedUser{user: user, at: now}
	return nil
}
