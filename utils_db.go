package main

import (
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Get struct {
	Name     string `gorm:"primaryKey"`
	Title    string
	Type     string
	Data     string
	Caption  string
	Creator  int64
	Entities []byte
}

type AntiSpam struct {
	Text string `gorm:"primaryKey"`
	Type string
}

type PidorStats struct {
	Date   time.Time `gorm:"primaryKey"`
	UserID int64
}

type PidorList gotgbot.User

type Duelist struct {
	UserID int64 `gorm:"primaryKey"`
	Deaths int
	Kills  int
}

type Warn struct {
	UserID   int64 `gorm:"primaryKey"`
	Amount   int
	LastWarn time.Time
}

type Nope struct {
	Text string `gorm:"primaryKey"`
}

type Bets struct {
	UserID    int64  `gorm:"primaryKey"`
	Text      string `gorm:"primaryKey"`
	Timestamp int64  `gorm:"primaryKey"`
}

type StatsWords struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	Word      string
	ShortWord string
}

type Stats struct {
	ContextID    int64 `gorm:"primaryKey"`
	StatType     int64 `gorm:"primaryKey"`
	Count        int64
	DayTimestamp int64 `gorm:"primaryKey"`
	LastUpdate   int64 `gorm:"default:1685221200"`
}

type Salvation struct {
	Text string `gorm:"primaryKey"`
}

type Bless struct {
	Text string `gorm:"primaryKey"`
}

func DataBaseInit(dsn string) (gorm.DB, error) {
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newPerformanceDBLogger(botPerformance, Logger),
	},
	)
	if err != nil {
		return *database, err
	}

	//Create tables, if they not exists in DB
	err = database.AutoMigrate(&gotgbot.User{}, &Get{}, &Warn{}, &PidorStats{}, &PidorList{}, &Duelist{}, &Bless{}, &Salvation{}, &Nope{}, &Stats{}, &StatsWords{}, &Bets{})
	if err != nil {
		return *database, err
	}
	return *database, nil
}

func GetNope() string {
	var nope Nope
	DB.Model(Nope{}).Order("RANDOM()").First(&nope)
	return nope.Text
}

func GetBless() string {
	var bless Bless
	DB.Model(Bless{}).Order("RANDOM()").First(&bless)
	return bless.Text
}

func GetSalvation() string {
	var salvation Salvation
	DB.Model(Salvation{}).Order("RANDOM()").First(&salvation)
	return salvation.Text
}
