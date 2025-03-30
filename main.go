package main

import (
	"log"
	"sort"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
)

type commandList struct {
	command  gotgbot.BotCommand
	response handlers.Response
}

func main() {
	commandMemberList := []commandList{
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
		{gotgbot.BotCommand{Command: "isekai", Description: "устроиться в роскомнадзор"}, Blessing},
		{gotgbot.BotCommand{Command: "marco", Description: "поло"}, Marco},
		{gotgbot.BotCommand{Command: "me", Description: "аналог команды /me из IRC (/me пошел спать)"}, Me},
		{gotgbot.BotCommand{Command: "meow", Description: "получить гифку с котиком"}, Meow},
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
	}

	commandAdminList := []commandList{
		{gotgbot.BotCommand{Command: "getid", Description: "получить ID юзера"}, Getid},
		{gotgbot.BotCommand{Command: "kick", Description: "кикнуть кого-нибудь"}, Kick},
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
		{gotgbot.BotCommand{Command: "testrandom", Description: "протестировать рандом бота "}, TestRandom},
	}

	commandMemberArray := []gotgbot.BotCommand{}
	for i := range commandMemberList {
		BotDispatcher.AddHandler(handlers.NewCommand(commandMemberList[i].command.Command, commandMemberList[i].response))
		commandMemberArray = append(commandMemberArray, commandMemberList[i].command)
	}
	sort.Slice(commandMemberArray, func(i, j int) bool {
		return commandMemberArray[i].Command < commandMemberArray[j].Command
	})
	_, err := Bot.SetMyCommands(commandMemberArray, &gotgbot.SetMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllGroupChats{}})
	if err != nil {
		log.Fatal(err)
	}

	commandAdminArray := []gotgbot.BotCommand{}
	for i := range commandAdminList {
		BotDispatcher.AddHandler(handlers.NewCommand(commandAdminList[i].command.Command, commandAdminList[i].response))
		commandAdminArray = append(commandAdminArray, commandAdminList[i].command)
	}
	commandAdminArray = append(commandAdminArray, commandMemberArray...)
	sort.Slice(commandAdminArray, func(i, j int) bool {
		return commandAdminArray[i].Command < commandAdminArray[j].Command
	})
	_, err = Bot.SetMyCommands(commandAdminArray, &gotgbot.SetMyCommandsOpts{Scope: gotgbot.BotCommandScopeAllChatAdministrators{}})
	if err != nil {
		log.Fatal(err)
	}

	//non-command handles
	BotDispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("russianroulette_accept"), Accept))
	BotDispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("russianroulette_deny"), Deny))
	BotDispatcher.AddHandler(handlers.NewChatMember(nil, OnChatMember))
	BotDispatcher.AddHandler(handlers.NewMessage(nil, OnText))
	BotDispatcher.AddHandler(handlers.NewInlineQuery(nil, GetInline))

	BotUpdater.Idle()
}
