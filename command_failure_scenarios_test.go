package main

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"
)

func failCreateTable(t *testing.T, table string, failure error) {
	t.Helper()
	if err := DB.Callback().Create().Before("gorm:create").Register("test:fail_write", func(tx *gorm.DB) {
		if tx.Statement.Table == table {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { DB.Callback().Create().Remove("test:fail_write") })
}

func TestWarningsConcurrentAndPartialFailures(t *testing.T) {
	t.Run("parallel threshold", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		DB.Create(&Warn{UserID: 42, Amount: 1, LastWarn: time.Now()})
		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs <- WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42))))
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		var row Warn
		DB.First(&row, 42)
		if row.Amount != 3 || len(client.callsFor("restrictChatMember")) != 1 || muteSeconds(t, 42) != 168*3600 {
			t.Fatalf("wrong threshold result: %+v", row)
		}
		if err := WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42)))); err != nil {
			t.Fatal(err)
		}
		if len(client.callsFor("restrictChatMember")) != 1 {
			t.Fatal("warning above threshold repeated restriction")
		}
	})
	t.Run("rejected restriction retries on new command", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		DB.Create(&Warn{UserID: 42, Amount: 2, LastWarn: time.Now()})
		failure := errors.New("restriction rejected")
		client.setError("restrictChatMember", failure)
		ctx := testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42)))
		if err := WarnUser(bot, ctx); !errors.Is(err, failure) {
			t.Fatalf("lost Telegram error: %v", err)
		}
		var row Warn
		DB.First(&row, 42)
		if row.Amount != 2 || muteSeconds(t, 42) != 0 || strings.Contains(strings.Join(client.sentTexts(), ""), "замьючен") {
			t.Fatal("rejection recorded as success")
		}
		client.setError("restrictChatMember", nil)
		if err := WarnUser(bot, ctx); err != nil {
			t.Fatal(err)
		}
		if len(client.callsFor("restrictChatMember")) != 2 || muteSeconds(t, 42) != 168*3600 {
			t.Fatal("retry did not apply exactly once")
		}
	})
	t.Run("counter write fails before restriction", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		failure := errors.New("warning write failed")
		failCreateTable(t, "warns", failure)
		if err := WarnUser(bot, testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42)))); !errors.Is(err, failure) {
			t.Fatal(err)
		}
		if len(client.callsFor("restrictChatMember")) != 0 || len(client.sentTexts()) != 0 {
			t.Fatal("action/notice sent after failed counter write")
		}
	})
	t.Run("final notification fails after mute", func(t *testing.T) {
		bot, client := setupTestEnv(t)
		DB.Create(&Warn{UserID: 42, Amount: 2, LastWarn: time.Now()})
		failure := errors.New("send failed")
		client.setError("sendMessage", failure)
		ctx := testReplyContext(bot, "/warn", testUser(100), testMessage("target", testUser(42)))
		if err := WarnUser(bot, ctx); !errors.Is(err, failure) {
			t.Fatal(err)
		}
		client.setError("sendMessage", nil)
		if err := WarnUser(bot, ctx); err != nil {
			t.Fatal(err)
		}
		var row Warn
		DB.First(&row, 42)
		if row.Amount != 3 || len(client.callsFor("restrictChatMember")) != 1 || muteSeconds(t, 42) != 168*3600 {
			t.Fatal("partial delivery repeated completed mute")
		}
	})
}

func gameFailureHandler(name string, bot *gotgbot.Bot) func() error {
	if name == "kill" {
		return func() error {
			return Kill(bot, testReplyContext(bot, "/kill", testUser(100), testMessage("target", testUser(42))))
		}
	}
	return func() error {
		return blessingWithSalvationRoll(bot, testContext(bot, "/blessing", testUser(42)), func() int { return 99 })
	}
}

func TestGameCommandFailures(t *testing.T) {
	for _, name := range []string{"kill", "blessing"} {
		t.Run(name, func(t *testing.T) {
			for _, phase := range []string{"counter", "death statistic", "restriction", "false restriction", "notification", "mute statistic"} {
				t.Run(phase, func(t *testing.T) {
					bot, client := setupTestEnv(t)
					failure := errors.New("injected " + phase)
					switch phase {
					case "counter":
						failCreateTable(t, "duelists", failure)
					case "death statistic":
						failCreateTable(t, "stats", failure)
					case "restriction":
						client.setError("restrictChatMember", failure)
					case "false restriction":
						client.setResult("restrictChatMember", []byte(`false`))
					case "notification":
						client.setError("sendMessage", failure)
					case "mute statistic":
						if err := DB.Callback().Create().Before("gorm:create").Register("test:fail_mute_stats", func(tx *gorm.DB) {
							if row, ok := tx.Statement.Dest.(*Stats); ok && row.StatType == 7 {
								tx.AddError(failure)
							}
						}); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { DB.Callback().Create().Remove("test:fail_mute_stats") })
					}
					if err := gameFailureHandler(name, bot)(); err == nil {
						t.Fatal("lost failure")
					}
					restrictions := len(client.callsFor("restrictChatMember"))
					if phase == "counter" || phase == "death statistic" {
						if restrictions != 0 {
							t.Fatal("restriction after failed persistence")
						}
					} else if restrictions != 1 {
						t.Fatal("unexpected automatic restriction retry")
					}
					var deaths int64
					DB.Model(&Duelist{}).Select("COALESCE(SUM(deaths),0)").Scan(&deaths)
					wantDeaths := int64(1)
					if phase == "counter" || phase == "death statistic" {
						wantDeaths = 0
					}
					if deaths != wantDeaths || statTotal(t, 8, 42) != wantDeaths {
						t.Fatalf("partial counters: lifetime=%d daily=%d want=%d", deaths, statTotal(t, 8, 42), wantDeaths)
					}
					if phase != "notification" && muteSeconds(t, 42) != 0 {
						t.Fatal("unconfirmed/unrecorded mute counted")
					}
					if phase == "notification" && muteSeconds(t, 42) <= 0 {
						t.Fatal("successful mute lost on send failure")
					}
					if phase != "notification" && len(client.callsFor("sendMessage")) != 0 {
						t.Fatal("false success notification")
					}
				})
			}
		})
	}
}

func TestAtomicDuelistCountersPreserveOtherFields(t *testing.T) {
	setupTestEnv(t)
	pool, _ := DB.DB()
	pool.SetMaxOpenConns(1)
	DB.Create(&Duelist{UserID: 42, Deaths: 7, Kills: 9})
	errs := make(chan error, 40)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			column := "deaths"
			if i%2 == 0 {
				column = "kills"
			}
			_, err := addDuelistCounter(42, column)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var row Duelist
	DB.First(&row, 42)
	if row.Deaths != 27 || row.Kills != 29 {
		t.Fatalf("lost concurrent increment: %+v", row)
	}
}

func TestDuelFinalDeliveryFailureDoesNotReplay(t *testing.T) {
	bot, client, ctx := duelScenario(t, 777)
	failure := errors.New("final edit failed")
	client.Observe = func(method string, _ map[string]any) {
		if method == "restrictChatMember" {
			client.Errors = map[string]error{"editMessageText": failure}
		}
	}
	if err := acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if len(client.callsFor("restrictChatMember")) != 1 || muteSeconds(t, 777) != 60 {
		t.Fatal("successful action lost")
	}
	var winner Duelist
	DB.First(&winner, 42)
	if winner.Kills != 1 {
		t.Fatal("winner lost on final edit failure")
	}
	if err := Accept(bot, ctx); err != nil {
		t.Fatal(err)
	}
	if len(client.callsFor("restrictChatMember")) != 1 {
		t.Fatal("replayed completed duel after delivery failure")
	}
}

// Assert the test handler still fits the actual command handler contract.
var _ func(*gotgbot.Bot, *ext.Context) error = WarnUser

func TestDuelCounterWriteFailureStopsRestriction(t *testing.T) {
	bot, client, ctx := duelScenario(t, 777)
	failure := errors.New("daily counter write failed")
	failCreateTable(t, "stats", failure)
	if err := acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	assertDuelReleased(t)
	var count int64
	DB.Model(&Duelist{}).Count(&count)
	if count != 0 || len(client.callsFor("restrictChatMember")) != 0 {
		t.Fatal("partially committed game counter/restriction")
	}
}
