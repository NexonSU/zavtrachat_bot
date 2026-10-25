package main

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func duelScenario(t *testing.T, victimID int64) (*gotgbot.Bot, *fakeBotClient, *ext.Context) {
	t.Helper()
	bot, client := setupTestEnv(t)
	resetDuelState(t)
	msg := testMessage("приглашение", &bot.User)
	msg.MessageId = 555
	msg.Entities = []gotgbot.MessageEntity{{Type: "text_mention", User: testUser(victimID)}, {Type: "text_mention", User: testUser(42)}}
	seedDuelInvitation(msg)
	return bot, client, testCallbackContext(bot, msg, testUser(victimID))
}

func duelRandom(order, bullet int) func(int, int) int {
	return func(min, max int) int {
		switch {
		case min == 1 && max == 360:
			return order
		case min == 1 && max == 5:
			return bullet
		default:
			return min
		}
	}
}

func noDuelPause(time.Duration) {}

func assertDuelReleased(t *testing.T) {
	t.Helper()
	duelMu.Lock()
	defer duelMu.Unlock()
	if busy["russianroulette"] || busy["russianroulettePending"] || busy["russianrouletteInProgress"] {
		t.Fatalf("game state not released: %v", busy)
	}
}

func TestDuelDeterministicOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		victim     int64
		order      int
		bullet     int
		statuses   map[int64]string
		ricochet   bool
		wantVictim int64
		wantWinner int64
		wantDraw   bool
	}{
		{name: "first shot", victim: 777, order: 1, bullet: 1, wantVictim: 777, wantWinner: 42},
		{name: "survival round", victim: 777, order: 1, bullet: 2, wantVictim: 42, wantWinner: 777},
		{name: "reversed order", victim: 777, order: 2, bullet: 1, wantVictim: 42, wantWinner: 777},
		{name: "configured admin retaliation", victim: 100, order: 1, bullet: 1, wantVictim: 42},
		{name: "Telegram admin draw", victim: 777, order: 1, bullet: 1, statuses: map[int64]string{777: "administrator"}, wantDraw: true},
		{name: "both admins without bystander", victim: 777, order: 1, bullet: 1, statuses: map[int64]string{42: "creator", 777: "administrator"}, wantDraw: true},
		{name: "both admins ricochet", victim: 777, order: 1, bullet: 1, statuses: map[int64]string{42: "creator", 777: "administrator"}, ricochet: true, wantVictim: 990, wantWinner: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, client, ctx := duelScenario(t, tc.victim)
			for id, status := range tc.statuses {
				client.setMemberStatus(id, status)
			}
			if tc.ricochet {
				DB.Create(&Stats{StatType: 3, ContextID: 990, DayTimestamp: GetStartOfDay(), Count: 1})
				DB.Create(&Duelist{UserID: 990, Deaths: 4})
			}
			if err := acceptDuelWith(bot, ctx, duelRandom(tc.order, tc.bullet), noDuelPause); err != nil {
				t.Fatal(err)
			}
			assertDuelReleased(t)
			restrictions := client.callsFor("restrictChatMember")
			if tc.wantDraw {
				edits := client.callsFor("editMessageText")
				if len(restrictions) != 0 || len(edits) == 0 || !strings.Contains(edits[len(edits)-1].Params["text"].(string), "ничья") {
					t.Fatal("draw must not restrict anyone")
				}
				return
			}
			if len(restrictions) != 1 || restrictions[0].Params["user_id"] != tc.wantVictim {
				t.Fatalf("wrong restricted participant: %+v", restrictions)
			}
			if statTotal(t, 8, tc.wantVictim) != 1 {
				t.Fatal("death must be recorded once for the actual victim")
			}
			if tc.wantWinner != 0 {
				var row Duelist
				if err := DB.First(&row, tc.wantWinner).Error; err != nil || row.Kills != 1 {
					t.Fatalf("winner count: %+v, %v", row, err)
				}
			}
			if tc.ricochet && (muteSeconds(t, 990) != 60 || Message.Entities[0].User.Id != 777 || bot.Id != 1) {
				t.Fatal("ricochet must retain its one-minute rule and must not mutate the invitation or bot user")
			}
		})
	}
}

func TestDuelConcurrentRequestsKeepOneInvitation(t *testing.T) {
	bot, client := setupTestEnv(t)
	resetDuelState(t)
	start := make(chan struct{})
	errs := make(chan error, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- Request(bot, testReplyContext(bot, "/duel", testUser(42), testMessage("target", testUser(777))))
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
	invitations := 0
	for _, call := range client.callsFor("sendMessage") {
		if strings.Contains(call.Params["text"].(string), "вызывает тебя на дуэль") {
			invitations++
		}
	}
	if invitations != 1 || len(client.callsFor("deleteMessage")) != 1 || Message.MessageId != 555 {
		t.Fatal("parallel requests created more than one invitation")
	}
	if err := Request(bot, testContext(bot, "/duel", testUser(42))); err != nil || len(client.callsFor("editMessageText")) != 0 {
		t.Fatal("fresh invitation was expired immediately")
	}
}

func TestDuelConcurrentAcceptsAndLateButtons(t *testing.T) {
	bot, client, ctx := duelScenario(t, 777)
	start := make(chan struct{})
	errs := make(chan error, 16)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause)
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
	if len(client.callsFor("restrictChatMember")) != 1 || statTotal(t, 8, 777) != 1 || len(client.callsFor("answerCallbackQuery")) != 16 {
		t.Fatal("repeated accepts ran the game more than once")
	}
	edits := len(client.callsFor("editMessageText"))
	if err := Deny(bot, ctx); err != nil {
		t.Fatal(err)
	}
	if len(client.callsFor("editMessageText")) != edits {
		t.Fatal("late refusal overwrote the completed game")
	}
	assertDuelReleased(t)
}

func TestDuelAcceptDenyRace(t *testing.T) {
	for range 8 {
		bot, client, ctx := duelScenario(t, 777)
		start := make(chan struct{})
		errs := make(chan error, 2)
		go func() { <-start; errs <- acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause) }()
		go func() { <-start; errs <- Deny(bot, ctx) }()
		close(start)
		for range 2 {
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		}
		if n := len(client.callsFor("restrictChatMember")); n > 1 || statTotal(t, 8, 777) != int64(n) {
			t.Fatal("accept/deny race produced inconsistent results")
		}
		assertDuelReleased(t)
	}
}

func TestDuelRejectsStaleExpiredAndMalformedCallbacks(t *testing.T) {
	for _, kind := range []string{"old message", "other chat", "expired", "missing participant", "missing message", "missing callback"} {
		t.Run(kind, func(t *testing.T) {
			bot, client, ctx := duelScenario(t, 777)
			switch kind {
			case "old message":
				ctx.EffectiveMessage.MessageId++
			case "other chat":
				ctx.EffectiveMessage.Chat.Id--
			case "expired":
				Message.Date = time.Now().Add(-61 * time.Second).Unix()
			case "missing participant":
				Message.Entities = nil
			case "missing message":
				ctx.EffectiveMessage = nil
			case "missing callback":
				ctx.CallbackQuery = nil
			}
			if err := acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause); err != nil {
				t.Fatal(err)
			}
			if len(client.callsFor("editMessageText")) != 0 || len(client.callsFor("restrictChatMember")) != 0 {
				t.Fatal("invalid callback changed the game")
			}
		})
	}
}

func TestDuelFailuresReleaseStateAndRejectReplay(t *testing.T) {
	for _, method := range []string{"answerCallbackQuery", "editMessageText", "getChatMember", "restrictChatMember"} {
		t.Run(method, func(t *testing.T) {
			bot, client, ctx := duelScenario(t, 777)
			failure := errors.New("Telegram unavailable")
			client.setError(method, failure)
			if err := acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause); !errors.Is(err, failure) {
				t.Fatalf("lost failure: %v", err)
			}
			assertDuelReleased(t)
			client.mu.Lock()
			delete(client.Errors, method)
			client.mu.Unlock()
			restrictions := len(client.callsFor("restrictChatMember"))
			deaths := statTotal(t, 8, 777)
			if err := acceptDuelWith(bot, ctx, duelRandom(1, 1), noDuelPause); err != nil {
				t.Fatal(err)
			}
			if len(client.callsFor("restrictChatMember")) != restrictions || statTotal(t, 8, 777) != deaths {
				t.Fatal("replayed callback repeated a partially completed action")
			}
		})
	}
}

func TestDuelTimeoutEditsInvitationAndKeepsRunningGame(t *testing.T) {
	bot, client, _ := duelScenario(t, 777)
	Message.Date = time.Now().Add(-61 * time.Second).Unix()
	ctx := testContext(bot, "/duel", testUser(42))
	ctx.EffectiveMessage.Chat.Id = -999
	if err := Request(bot, ctx); err != nil {
		t.Fatal(err)
	}
	edits := client.callsFor("editMessageText")
	if len(edits) != 1 || edits[0].Params["chat_id"] != int64(-100) || edits[0].Params["message_id"] != int64(555) {
		t.Fatal("timeout edited the new command instead of the invitation")
	}
	assertDuelReleased(t)
	client.clearCalls()
	busy["russianrouletteInProgress"] = true
	Message.Date = time.Now().Add(-121 * time.Second).Unix()
	if err := Request(bot, ctx); err != nil || !busy["russianrouletteInProgress"] || len(client.callsFor("deleteMessage")) != 0 {
		t.Fatal("a new request must not interrupt a running handler")
	}
}

func TestDuelInvitationAndRefusalFailures(t *testing.T) {
	for _, method := range []string{"sendMessage", "answerCallbackQuery", "editMessageText"} {
		t.Run(method, func(t *testing.T) {
			bot, client, ctx := duelScenario(t, 777)
			failure := errors.New("Telegram failure")
			client.setError(method, failure)
			var err error
			if method == "sendMessage" {
				finishDuel()
				err = Request(bot, testReplyContext(bot, "/duel", testUser(42), testMessage("target", testUser(777))))
			} else {
				err = Deny(bot, ctx)
			}
			if !errors.Is(err, failure) {
				t.Fatalf("failure lost: %v", err)
			}
			assertDuelReleased(t)
			if len(client.callsFor("restrictChatMember")) != 0 {
				t.Fatal("failed invitation or refusal restricted a participant")
			}
		})
	}
}

func TestDuelRunningGameCannotBeReplaced(t *testing.T) {
	bot, client, ctx := duelScenario(t, 777)
	entered, release := make(chan struct{}), make(chan struct{})
	finished := make(chan error, 1)
	var enterOnce, releaseOnce sync.Once
	go func() {
		finished <- acceptDuelWith(bot, ctx, duelRandom(1, 1), func(time.Duration) {
			enterOnce.Do(func() { close(entered) })
			<-release
		})
	}()
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("game never reached its animation")
	}
	duelMu.Lock()
	Message.Date = time.Now().Add(-121 * time.Second).Unix()
	duelMu.Unlock()
	requestErr := Request(bot, testReplyContext(bot, "/duel", testUser(50), testMessage("target", testUser(888))))
	denyErr := Deny(bot, ctx)
	releaseOnce.Do(func() { close(release) })
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if requestErr != nil || denyErr != nil {
		t.Fatalf("request/refusal failed: %v / %v", requestErr, denyErr)
	}
	if len(client.callsFor("deleteMessage")) != 0 || len(client.callsFor("restrictChatMember")) != 1 || statTotal(t, 8, 777) != 1 {
		t.Fatal("another command or late refusal replaced a running game")
	}
	assertDuelReleased(t)
}
