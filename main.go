package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/reaction"
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
var Logger *slog.Logger

func main() {
	Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(Logger)
	slog.Info("init: config")
	err := ConfigInit()
	if err != nil {
		slog.Error(fmt.Sprintf("config init failed: %s", err))
		panic(fmt.Errorf("config init failed: %s", err))
	}
	slog.Info("init: database")
	DB, err = DataBaseInit(Config.DSN)
	if err != nil {
		slog.Error(fmt.Sprintf("database init failed: %s", err))
		panic(fmt.Errorf("database init failed: %s", err))
	}
	slog.Info("init: bot")
	err = BotInit()
	if err != nil {
		slog.Error(fmt.Sprintf("bot init failed: %s", err))
		panic(fmt.Errorf("bot init failed: %s", err))
	}
	slog.Info("init: ai")
	err = AiInit()
	if err != nil {
		slog.Error(fmt.Sprintf("ai init failed: %s", err))
		fmt.Printf("ai init failed: %s", err)
	}
	commandList := []commandList{
		{gotgbot.BotCommand{Command: "releases", Description: "список релизов", IsEphemeral: true}, Releases},
		{gotgbot.BotCommand{Command: "russianroulette", Description: "вызвать на дуэль кого-нибудь", IsEphemeral: true}, Request},
		{gotgbot.BotCommand{Command: "savetopm", Description: "сохранить пост в личку", IsEphemeral: true}, SaveToPM},
		{gotgbot.BotCommand{Command: "sed", Description: "заменить текст типо как в sed", IsEphemeral: true}, Sed},
		{gotgbot.BotCommand{Command: "khaleesi", Description: "заменить текст типо как кхалиси мем", IsEphemeral: true}, Khaleesi},
		{gotgbot.BotCommand{Command: "set", Description: "сохранить гет", IsEphemeral: true}, Set},
		{gotgbot.BotCommand{Command: "shrug", Description: "¯\\_(ツ)_/¯", IsEphemeral: true}, Shrug},
		{gotgbot.BotCommand{Command: "stats", Description: "статистика чата", IsEphemeral: true}, StatsLinks},
		{gotgbot.BotCommand{Command: "get", Description: "получить гет", IsEphemeral: true}, GetGet},
		{gotgbot.BotCommand{Command: "getall", Description: "получить список гетов", IsEphemeral: true}, Getall},
		{gotgbot.BotCommand{Command: "giveme", Description: "сохранить пост в личку", IsEphemeral: true}, SaveToPM},
		{gotgbot.BotCommand{Command: "google", Description: "загуглить что-нибудь", IsEphemeral: true}, Google},
		{gotgbot.BotCommand{Command: "hug", Description: "обнять кого-нибудь", IsEphemeral: true}, Hug},
		{gotgbot.BotCommand{Command: "isekai", Description: "попасть в исекай", IsEphemeral: true}, Blessing},
		{gotgbot.BotCommand{Command: "isekaitop", Description: "топ исекая чата", IsEphemeral: true}, BlessingTop},
		{gotgbot.BotCommand{Command: "marco", Description: "поло", IsEphemeral: true}, Marco},
		{gotgbot.BotCommand{Command: "ai", Description: "причина твоей переплаты за оперативу", IsEphemeral: true}, AI},
		{gotgbot.BotCommand{Command: "restartai", Description: "перезапуск причины переплаты за оперативу", IsEphemeral: true}, RestartAI},
		{gotgbot.BotCommand{Command: "setaisystem", Description: "сменить системный промпт причины переплаты за оперативу", IsEphemeral: true}, SetAISystem},
		{gotgbot.BotCommand{Command: "date", Description: "вернуть дату и время сервера", IsEphemeral: true}, Date},
		{gotgbot.BotCommand{Command: "me", Description: "аналог команды /me из IRC (/me пошел спать)", IsEphemeral: true}, Me},
		{gotgbot.BotCommand{Command: "mp3", Description: "скачать музыку по ссылке", IsEphemeral: true}, Mp3},
		{gotgbot.BotCommand{Command: "meme", Description: "получить мем", IsEphemeral: true}, Meme},
		{gotgbot.BotCommand{Command: "redemption", Description: "возродить всех недавних юзеров", IsEphemeral: true}, Redemption},
		{gotgbot.BotCommand{Command: "massrevive", Description: "возродить всех недавних юзеров", IsEphemeral: true}, Redemption},
		{gotgbot.BotCommand{Command: "megarevive", Description: "возродить всех недавних юзеров", IsEphemeral: true}, Redemption},
		{gotgbot.BotCommand{Command: "meow", Description: "получить гифку с котиком", IsEphemeral: true}, Meow},
		{gotgbot.BotCommand{Command: "woof", Description: "получить гифку с не котиком", IsEphemeral: true}, Woof},
		{gotgbot.BotCommand{Command: "mlem", Description: "получить гифку с котиком", IsEphemeral: true}, Meow},
		{gotgbot.BotCommand{Command: "mywarns", Description: "посмотреть количество своих предупреждений", IsEphemeral: true}, Mywarns},
		{gotgbot.BotCommand{Command: "pidor", Description: "запустить игру \"Пидор Дня!\"", IsEphemeral: true}, Pidor},
		{gotgbot.BotCommand{Command: "pidorall", Description: "статистика \"Пидор Дня!\" за всё время", IsEphemeral: true}, Pidorall},
		{gotgbot.BotCommand{Command: "pidoreg", Description: "зарегистрироваться в \"Пидор Дня!\"", IsEphemeral: true}, Pidoreg},
		{gotgbot.BotCommand{Command: "pidorme", Description: "личная статистика \"Пидор Дня!\"", IsEphemeral: true}, Pidorme},
		{gotgbot.BotCommand{Command: "pidorstats", Description: "статистика \"Пидор Дня!\" за год", IsEphemeral: true}, Pidorstats},
		{gotgbot.BotCommand{Command: "pidorules", Description: "правила \"Пидор Дня!\"", IsEphemeral: true}, Pidorules},
		{gotgbot.BotCommand{Command: "anekdot", Description: "получить рандомный анекдот с anekdot.ru", IsEphemeral: true}, Anekdot},
		{gotgbot.BotCommand{Command: "blessing", Description: "устроиться в роскомнадзор", IsEphemeral: true}, Blessing},
		{gotgbot.BotCommand{Command: "blessingtop", Description: "топ роскомнадзоров чата", IsEphemeral: true}, BlessingTop},
		{gotgbot.BotCommand{Command: "bonk", Description: "бонкнуть кого-нибудь", IsEphemeral: true}, Bonk},
		{gotgbot.BotCommand{Command: "finduserinmessage", Description: "найти юзера в сообщении", IsEphemeral: true}, FindUserInMessageTest},
		{gotgbot.BotCommand{Command: "cur", Description: "посмотреть курс валют", IsEphemeral: true}, Cur},
		{gotgbot.BotCommand{Command: "del", Description: "удалить гет", IsEphemeral: true}, Del},
		{gotgbot.BotCommand{Command: "distort", Description: "переебать медиа", IsEphemeral: true}, Distort},
		{gotgbot.BotCommand{Command: "invert", Description: "инвертировать медиа", IsEphemeral: true}, Invert},
		{gotgbot.BotCommand{Command: "reverse", Description: "инвертировать медиа", IsEphemeral: true}, Invert},
		{gotgbot.BotCommand{Command: "loop", Description: "залупить гифку", IsEphemeral: true}, Loop},
		{gotgbot.BotCommand{Command: "duel", Description: "вызвать на дуэль кого-нибудь", IsEphemeral: true}, Request},
		{gotgbot.BotCommand{Command: "duelstats", Description: "посмотреть свою статистику дуэли", IsEphemeral: true}, Duelstats},
		{gotgbot.BotCommand{Command: "ping", Description: "понг", IsEphemeral: true}, Ping},
		{gotgbot.BotCommand{Command: "tldr", Description: "получить от яндекса пересказ по ссылке", IsEphemeral: true}, TLDR},
		{gotgbot.BotCommand{Command: "slap", Description: "дать леща кому-нибудь", IsEphemeral: true}, Slap},
		{gotgbot.BotCommand{Command: "suicide", Description: "устроиться в роскомнадзор", IsEphemeral: true}, Blessing},
		{gotgbot.BotCommand{Command: "topm", Description: "сохранить пост в личку", IsEphemeral: true}, SaveToPM},
		{gotgbot.BotCommand{Command: "advice", Description: "получить совет", IsEphemeral: true}, Advice},
		{gotgbot.BotCommand{Command: "bet", Description: "поставить ставку", IsEphemeral: true}, Bet},
		{gotgbot.BotCommand{Command: "allbets", Description: "список актуальных ставок", IsEphemeral: true}, AllBets},
		{gotgbot.BotCommand{Command: "delbet", Description: "удалить ставку", IsEphemeral: true}, DelBet},
		{gotgbot.BotCommand{Command: "convert", Description: "конвертировать файл, доп.параметры: mp3,ogg,gif,audio,voice,animation", IsEphemeral: true}, Convert},
		{gotgbot.BotCommand{Command: "download", Description: "скачать файл", IsEphemeral: true}, Download},
		{gotgbot.BotCommand{Command: "wget", Description: "скачать файл", IsEphemeral: true}, Download},
		{gotgbot.BotCommand{Command: "getid", Description: "получить ID юзера", IsEphemeral: true}, Getid},
		{gotgbot.BotCommand{Command: "kick", Description: "кикнуть кого-нибудь", IsEphemeral: true}, Kick},
		{gotgbot.BotCommand{Command: "gigabite", Description: "укусить чат", IsEphemeral: true}, Shotgun},
		{gotgbot.BotCommand{Command: "gigakill", Description: "выстрелить из шотгана", IsEphemeral: true}, Shotgun},
		{gotgbot.BotCommand{Command: "ultrakill", Description: "выстрелить из ультрашотгана", IsEphemeral: true}, Shotgun},
		{gotgbot.BotCommand{Command: "shotgun", Description: "выстрелить из шотгана", IsEphemeral: true}, Shotgun},
		{gotgbot.BotCommand{Command: "bite", Description: "укусить кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "kilobite", Description: "сильно укусить кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "kilomlem", Description: "сильно укусить кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "megabite", Description: "очень сильно укусить кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "megamlem", Description: "очень сильно укусить кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "kill", Description: "пристрелить кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "mute", Description: "заглушить кого-нибудь", IsEphemeral: true}, Mute},
		{gotgbot.BotCommand{Command: "pidordel", Description: "удалить игрока из \"Пидор Дня!\"", IsEphemeral: true}, Pidordel},
		{gotgbot.BotCommand{Command: "pidorremovetoday", Description: "удалить сегодняшнего \"Пидор Дня!\"", IsEphemeral: true}, PidorRemoveToday},
		{gotgbot.BotCommand{Command: "pidorlist", Description: "список всех игроков \"Пидор Дня!\"", IsEphemeral: true}, Pidorlist},
		{gotgbot.BotCommand{Command: "restart", Description: "перезапуск бота", IsEphemeral: true}, Restart},
		{gotgbot.BotCommand{Command: "resurrect", Description: "возродить кого-нибудь", IsEphemeral: true}, Revive},
		{gotgbot.BotCommand{Command: "revive", Description: "возродить кого-нибудь", IsEphemeral: true}, Revive},
		{gotgbot.BotCommand{Command: "addbless", Description: "добавить причину блесса", IsEphemeral: true}, AddBless},
		{gotgbot.BotCommand{Command: "addnope", Description: "добавить сообщение отказа по кнопке", IsEphemeral: true}, AddNope},
		{gotgbot.BotCommand{Command: "ban", Description: "забанить кого-нибудь", IsEphemeral: true}, Ban},
		{gotgbot.BotCommand{Command: "bless", Description: "попросить помолчать кого-нибудь", IsEphemeral: true}, Kill},
		{gotgbot.BotCommand{Command: "debug", Description: "получить сообщение в виде JSON", IsEphemeral: true}, Debug},
		{gotgbot.BotCommand{Command: "debuguser", Description: "получить сообщение в виде JSON", IsEphemeral: true}, DebugUser},
		{gotgbot.BotCommand{Command: "say", Description: "заставить бота сказать что-нибудь", IsEphemeral: true}, Say},
		{gotgbot.BotCommand{Command: "setgetowner", Description: "задать владельца гета", IsEphemeral: true}, SetGetOwner},
		{gotgbot.BotCommand{Command: "unban", Description: "разбанить кого-нибудь", IsEphemeral: true}, Unban},
		{gotgbot.BotCommand{Command: "unmute", Description: "разглушить кого-нибудь", IsEphemeral: true}, Unmute},
		{gotgbot.BotCommand{Command: "warn", Description: "предупредить кого-нибудь", IsEphemeral: true}, WarnUser},
		{gotgbot.BotCommand{Command: "testrandom", Description: "протестировать рандом бота", IsEphemeral: true}, TestRandom},
		{gotgbot.BotCommand{Command: "remove", Description: "удалить сообщение", IsEphemeral: true}, RemoveReplyMessage},
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
	_, err = Bot.SetMyCommands(commandArray, &gotgbot.SetMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllGroupChats{}})
	if err != nil {
		slog.Error(err.Error())
	}

	//non-command handles
	BotDispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("russianroulette_accept"), Accept))
	BotDispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("russianroulette_deny"), Deny))
	BotDispatcher.AddHandler(handlers.Message{Response: RemoveJoinMessageAndJoinUser, Filter: message.ChatID(Config.ReserveChat)})
	BotDispatcher.AddHandler(handlers.Message{AllowChannel: true, Response: ForwardChannelPost, Filter: message.ChatID(Config.Channel)})
	BotDispatcher.AddHandler(handlers.NewMessage(nil, OnText))
	BotDispatcher.AddHandler(handlers.NewInlineQuery(nil, GetInline))
	BotDispatcher.AddHandler(handlers.NewReaction(reaction.ChatID(Config.Chat), OnReaction))

	go gotdClientInit()

	BotUpdater.Idle()
}
