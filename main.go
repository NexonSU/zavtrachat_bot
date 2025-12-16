package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/gotd/td/telegram"
	"gorm.io/gorm"
)

type commandList struct {
	command  gotgbot.BotCommand
	response handlers.Response
}

var Config *Configuration
var DB gorm.DB
var HTTPClientProxy func(*http.Request) (*url.URL, error)
var Bot *gotgbot.Bot
var BotDispatcher *ext.Dispatcher
var BotUpdater *ext.Updater
var GotdClient *telegram.Client
var GotdContext context.Context

func main() {
	log.Println("init: config")
	err := ConfigInit()
	if err != nil {
		panic(fmt.Errorf("config init failed: %s", err))
	}
	log.Println("init: database")
	DB, err = DataBaseInit(Config.DSN)
	if err != nil {
		panic(fmt.Errorf("database init failed: %s", err))
	}
	log.Println("init: bot")
	err = BotInit()
	if err != nil {
		panic(fmt.Errorf("bot init failed: %s", err))
	}
	commandList := []commandList{
		{gotgbot.BotCommand{Command: "releases", Description: "список релизов"}, Releases},
		{gotgbot.BotCommand{Command: "russianroulette", Description: "вызвать на дуэль кого-нибудь"}, Request},
		{gotgbot.BotCommand{Command: "savetopm", Description: "сохранить пост в личку"}, SaveToPM},
		{gotgbot.BotCommand{Command: "sed", Description: "заменить текст типо как в sed"}, Sed},
		{gotgbot.BotCommand{Command: "khaleesi", Description: "заменить текст типо как кхалиси мем"}, Khaleesi},
		{gotgbot.BotCommand{Command: "set", Description: "сохранить гет"}, Set},
		{gotgbot.BotCommand{Command: "shrug", Description: "¯\\_(ツ)_/¯"}, Shrug},
		{gotgbot.BotCommand{Command: "stats", Description: "статистика чата"}, StatsLinks},
		{gotgbot.BotCommand{Command: "get", Description: "получить гет"}, GetGet},
		{gotgbot.BotCommand{Command: "getall", Description: "получить список гетов"}, Getall},
		{gotgbot.BotCommand{Command: "giveme", Description: "сохранить пост в личку"}, SaveToPM},
		{gotgbot.BotCommand{Command: "google", Description: "загуглить что-нибудь"}, Google},
		{gotgbot.BotCommand{Command: "hug", Description: "обнять кого-нибудь"}, Hug},
		{gotgbot.BotCommand{Command: "isekai", Description: "попасть в исекай"}, Blessing},
		{gotgbot.BotCommand{Command: "isekaitop", Description: "топ исекая чата"}, BlessingTop},
		{gotgbot.BotCommand{Command: "marco", Description: "поло"}, Marco},
		{gotgbot.BotCommand{Command: "ai", Description: "ИИшечка, поддерживаются текст и картинки"}, AI},
		{gotgbot.BotCommand{Command: "date", Description: "вернуть дату и время сервера"}, Date},
		{gotgbot.BotCommand{Command: "me", Description: "аналог команды /me из IRC (/me пошел спать)"}, Me},
		{gotgbot.BotCommand{Command: "mp3", Description: "скачать музыку по ссылке"}, Mp3},
		{gotgbot.BotCommand{Command: "meow", Description: "получить гифку с котиком"}, Meow},
		{gotgbot.BotCommand{Command: "woof", Description: "получить гифку с не котиком"}, Woof},
		{gotgbot.BotCommand{Command: "mlem", Description: "получить гифку с котиком"}, Meow},
		{gotgbot.BotCommand{Command: "mywarns", Description: "посмотреть количество своих предупреждений"}, Mywarns},
		{gotgbot.BotCommand{Command: "pidor", Description: "запустить игру \"Пидор Дня!\""}, Pidor},
		{gotgbot.BotCommand{Command: "pidorall", Description: "статистика \"Пидор Дня!\" за всё время"}, Pidorall},
		{gotgbot.BotCommand{Command: "pidoreg", Description: "зарегистрироваться в \"Пидор Дня!\""}, Pidoreg},
		{gotgbot.BotCommand{Command: "pidorme", Description: "личная статистика \"Пидор Дня!\""}, Pidorme},
		{gotgbot.BotCommand{Command: "pidorstats", Description: "статистика \"Пидор Дня!\" за год"}, Pidorstats},
		{gotgbot.BotCommand{Command: "pidorules", Description: "правила \"Пидор Дня!\""}, Pidorules},
		{gotgbot.BotCommand{Command: "anekdot", Description: "получить рандомный анекдот с anekdot.ru"}, Anekdot},
		{gotgbot.BotCommand{Command: "blessing", Description: "устроиться в роскомнадзор"}, Blessing},
		{gotgbot.BotCommand{Command: "blessingtop", Description: "топ роскомнадзоров чата"}, BlessingTop},
		{gotgbot.BotCommand{Command: "bonk", Description: "бонкнуть кого-нибудь"}, Bonk},
		{gotgbot.BotCommand{Command: "cur", Description: "посмотреть курс валют"}, Cur},
		{gotgbot.BotCommand{Command: "del", Description: "удалить гет"}, Del},
		{gotgbot.BotCommand{Command: "distort", Description: "переебать медиа"}, Distort},
		{gotgbot.BotCommand{Command: "invert", Description: "инвертировать медиа"}, Invert},
		{gotgbot.BotCommand{Command: "reverse", Description: "инвертировать медиа"}, Invert},
		{gotgbot.BotCommand{Command: "loop", Description: "залупить гифку"}, Loop},
		{gotgbot.BotCommand{Command: "duel", Description: "вызвать на дуэль кого-нибудь"}, Request},
		{gotgbot.BotCommand{Command: "duelstats", Description: "посмотреть свою статистику дуэли"}, Duelstats},
		{gotgbot.BotCommand{Command: "ping", Description: "понг"}, Ping},
		{gotgbot.BotCommand{Command: "tldr", Description: "получить от яндекса пересказ по ссылке"}, TLDR},
		{gotgbot.BotCommand{Command: "slap", Description: "дать леща кому-нибудь"}, Slap},
		{gotgbot.BotCommand{Command: "suicide", Description: "устроиться в роскомнадзор"}, Blessing},
		{gotgbot.BotCommand{Command: "topm", Description: "сохранить пост в личку"}, SaveToPM},
		{gotgbot.BotCommand{Command: "advice", Description: "получить совет"}, Advice},
		{gotgbot.BotCommand{Command: "bet", Description: "поставить ставку"}, Bet},
		{gotgbot.BotCommand{Command: "allbets", Description: "список актуальных ставок"}, AllBets},
		{gotgbot.BotCommand{Command: "delbet", Description: "удалить ставку"}, DelBet},
		{gotgbot.BotCommand{Command: "convert", Description: "конвертировать файл, доппараметры: mp3,ogg,gif,audio,voice,animation"}, Convert},
		{gotgbot.BotCommand{Command: "download", Description: "скачать файл"}, Download},
		{gotgbot.BotCommand{Command: "wget", Description: "скачать файл"}, Download},
		{gotgbot.BotCommand{Command: "getid", Description: "получить ID юзера"}, Getid},
		{gotgbot.BotCommand{Command: "kick", Description: "кикнуть кого-нибудь"}, Kick},
		{gotgbot.BotCommand{Command: "gigabite", Description: "укусить чат"}, Shotgun},
		{gotgbot.BotCommand{Command: "gigakill", Description: "выстрелить из шотгана"}, Shotgun},
		{gotgbot.BotCommand{Command: "shotgun", Description: "выстрелить из шотгана"}, Shotgun},
		{gotgbot.BotCommand{Command: "bite", Description: "укусить кого-нибудь"}, Kill},
		{gotgbot.BotCommand{Command: "kilobite", Description: "сильно укусить кого-нибудь"}, Kill},
		{gotgbot.BotCommand{Command: "megabite", Description: "очень сильно укусить кого-нибудь"}, Kill},
		{gotgbot.BotCommand{Command: "kill", Description: "пристрелить кого-нибудь"}, Kill},
		{gotgbot.BotCommand{Command: "mute", Description: "заглушить кого-нибудь"}, Mute},
		{gotgbot.BotCommand{Command: "pidordel", Description: "удалить игрока из \"Пидор Дня!\""}, Pidordel},
		{gotgbot.BotCommand{Command: "pidorlist", Description: "список всех игроков \"Пидор Дня!\""}, Pidorlist},
		{gotgbot.BotCommand{Command: "restart", Description: "перезапуск бота"}, Restart},
		{gotgbot.BotCommand{Command: "resurrect", Description: "возродить кого-нибудь"}, Revive},
		{gotgbot.BotCommand{Command: "revive", Description: "возродить кого-нибудь"}, Revive},
		{gotgbot.BotCommand{Command: "addbless", Description: "добавить причину блесса"}, AddBless},
		{gotgbot.BotCommand{Command: "addnope", Description: "добавить сообщение отказа по кнопке"}, AddNope},
		{gotgbot.BotCommand{Command: "ban", Description: "забанить кого-нибудь"}, Ban},
		{gotgbot.BotCommand{Command: "bless", Description: "попросить помолчать кого-нибудь"}, Kill},
		{gotgbot.BotCommand{Command: "debug", Description: "получить сообщение в виде JSON"}, Debug},
		{gotgbot.BotCommand{Command: "say", Description: "заставить бота сказать что-нибудь"}, Say},
		{gotgbot.BotCommand{Command: "setgetowner", Description: "задать владельца гета"}, SetGetOwner},
		{gotgbot.BotCommand{Command: "unban", Description: "разбанить кого-нибудь"}, Unban},
		{gotgbot.BotCommand{Command: "unmute", Description: "разглушить кого-нибудь"}, Unmute},
		{gotgbot.BotCommand{Command: "warn", Description: "предупредить кого-нибудь"}, WarnUser},
		{gotgbot.BotCommand{Command: "testrandom", Description: "протестировать рандом бота"}, TestRandom},
		{gotgbot.BotCommand{Command: "remove", Description: "удалить сообщение"}, RemoveReplyMessage},
	}

	Bot.DeleteMyCommands(&gotgbot.DeleteMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllPrivateChats{}})
	Bot.DeleteMyCommands(&gotgbot.DeleteMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllGroupChats{}})
	Bot.DeleteMyCommands(&gotgbot.DeleteMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllChatAdministrators{}})
	Bot.DeleteMyCommands(&gotgbot.DeleteMyCommandsOpts{Scope: gotgbot.BotCommandScopeDefault{}})

	commandArray := []gotgbot.BotCommand{}
	for i := range commandList {
		BotDispatcher.AddHandler(handlers.NewCommand(commandList[i].command.Command, commandList[i].response))
		commandArray = append(commandArray, commandList[i].command)
	}
	sort.Slice(commandArray, func(i, j int) bool {
		return commandArray[i].Command < commandArray[j].Command
	})
	_, err = Bot.SetMyCommands(commandArray, nil)
	if err != nil {
		log.Fatal(err)
	}

	//non-command  handles
	BotDispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("russianroulette_accept"), Accept))
	BotDispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("russianroulette_deny"), Deny))
	BotDispatcher.AddHandler(handlers.Message{Response: RemoveJoinMessageAndJoinUser, Filter: message.ChatID(Config.ReserveChat)})
	BotDispatcher.AddHandler(handlers.Message{AllowChannel: true, Response: ForwardPost, Filter: message.ChatID(Config.Channel)})
	BotDispatcher.AddHandler(handlers.NewMessage(nil, OnText))
	BotDispatcher.AddHandler(handlers.NewInlineQuery(nil, GetInline))

	go gotdClientInit()

	BotUpdater.Idle()
}
