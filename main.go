package main

import (
	"log"
	"sort"

	tele "gopkg.in/telebot.v3"
)

type commandList struct {
	command    tele.Command
	function   tele.HandlerFunc
	middleware tele.MiddlewareFunc
}

func main() {
	BotInit()

	//middle wares
	admin := Whitelist(append(Config.Admins, Config.SysAdmin)...)
	moder := Whitelist(append(append(Config.Admins, Config.Moders...), Config.SysAdmin)...)
	chats := Whitelist(append(append(Config.Admins, Config.Moders...), Config.SysAdmin, Config.Chat, Config.ReserveChat)...)
	//chatr := Whitelist(Config.ReserveChat)
	//chann := Whitelist(Config.Channel)

	commandMemberList := []commandList{
		{tele.Command{Text: "releases", Description: "список релизов"}, Releases, chats},
		{tele.Command{Text: "russianroulette", Description: "вызвать на дуэль кого-нибудь"}, Request, chats},
		{tele.Command{Text: "savetopm", Description: "сохранить пост в личку"}, SaveToPM, chats},
		{tele.Command{Text: "sed", Description: "заменить текст типо как в sed"}, Sed, chats},
		{tele.Command{Text: "khaleesi", Description: "заменить текст типо как кхалиси мем"}, Khaleesi, chats},
		{tele.Command{Text: "set", Description: "сохранить гет"}, Set, chats},
		{tele.Command{Text: "shrug", Description: "¯\\_(ツ)_/¯"}, Shrug, chats},
		{tele.Command{Text: "stats", Description: "статистика чата"}, StatsLinks, chats},
		{tele.Command{Text: "get", Description: "получить гет"}, GetGet, chats},
		{tele.Command{Text: "getall", Description: "получить список гетов"}, Getall, chats},
		{tele.Command{Text: "giveme", Description: "сохранить пост в личку"}, SaveToPM, chats},
		{tele.Command{Text: "google", Description: "загуглить что-нибудь"}, Google, chats},
		{tele.Command{Text: "hug", Description: "обнять кого-нибудь"}, Hug, chats},
		{tele.Command{Text: "isekai", Description: "устроиться в роскомнадзор"}, Blessing, chats},
		{tele.Command{Text: "marco", Description: "поло"}, Marco, chats},
		{tele.Command{Text: "me", Description: "аналог команды /me из IRC (/me пошел спать)"}, Me, chats},
		{tele.Command{Text: "meow", Description: "получить гифку с котиком"}, Meow, chats},
		{tele.Command{Text: "mlem", Description: "получить гифку с котиком"}, Meow, chats},
		{tele.Command{Text: "mywarns", Description: "посмотреть количество своих предупреждений"}, Mywarns, chats},
		{tele.Command{Text: "pidor", Description: "запустить игру \"Пидор Дня!\""}, Pidor, chats},
		{tele.Command{Text: "pidorall", Description: "статистика \"Пидор Дня!\" за всё время"}, Pidorall, chats},
		{tele.Command{Text: "pidoreg", Description: "зарегистрироваться в \"Пидор Дня!\""}, Pidoreg, chats},
		{tele.Command{Text: "pidorme", Description: "личная статистика \"Пидор Дня!\""}, Pidorme, chats},
		{tele.Command{Text: "pidorstats", Description: "статистика \"Пидор Дня!\" за год"}, Pidorstats, chats},
		{tele.Command{Text: "pidorules", Description: "правила \"Пидор Дня!\""}, Pidorules, chats},
		{tele.Command{Text: "anekdot", Description: "получить рандомный анекдот с anekdot.ru"}, Anekdot, chats},
		{tele.Command{Text: "blessing", Description: "устроиться в роскомнадзор"}, Blessing, chats},
		{tele.Command{Text: "bonk", Description: "бонкнуть кого-нибудь"}, Bonk, chats},
		{tele.Command{Text: "cur", Description: "посмотреть курс валют"}, Cur, chats},
		{tele.Command{Text: "del", Description: "удалить гет"}, Del, chats},
		{tele.Command{Text: "distort", Description: "переебать медиа"}, Distort, chats},
		{tele.Command{Text: "invert", Description: "инвертировать медиа"}, Invert, chats},
		{tele.Command{Text: "reverse", Description: "инвертировать медиа"}, Invert, chats},
		{tele.Command{Text: "loop", Description: "залупить гифку"}, Loop, chats},
		{tele.Command{Text: "duel", Description: "вызвать на дуэль кого-нибудь"}, Request, chats},
		{tele.Command{Text: "duelstats", Description: "посмотреть свою статистику дуэли"}, Duelstats, chats},
		{tele.Command{Text: "ping", Description: "понг"}, Ping, chats},
		{tele.Command{Text: "tldr", Description: "получить от яндекса пересказ по ссылке"}, TLDR, chats},
		{tele.Command{Text: "slap", Description: "дать леща кому-нибудь"}, Slap, chats},
		{tele.Command{Text: "suicide", Description: "устроиться в роскомнадзор"}, Blessing, chats},
		{tele.Command{Text: "topm", Description: "сохранить пост в личку"}, SaveToPM, chats},
		{tele.Command{Text: "advice", Description: "получить совет"}, Advice, chats},
		{tele.Command{Text: "bet", Description: "поставить ставку"}, Bet, chats},
		{tele.Command{Text: "allbets", Description: "список актуальных ставок"}, AllBets, chats},
		{tele.Command{Text: "delbet", Description: "удалить ставку"}, DelBet, chats},
		{tele.Command{Text: "convert", Description: "конвертировать файл, доппараметры: mp3,ogg,gif,audio,voice,animation"}, Convert, chats},
		{tele.Command{Text: "download", Description: "скачать файл"}, Download, chats},
		{tele.Command{Text: "wget", Description: "скачать файл"}, Download, chats},
	}

	commandAdminList := []commandList{
		{tele.Command{Text: "getid", Description: "получить ID юзера"}, Getid, moder},
		{tele.Command{Text: "kick", Description: "кикнуть кого-нибудь"}, Kick, moder},
		{tele.Command{Text: "bite", Description: "укусить кого-нибудь"}, Kill, moder},
		{tele.Command{Text: "kilobite", Description: "сильно укусить кого-нибудь"}, Kill, moder},
		{tele.Command{Text: "megabite", Description: "очень сильно укусить кого-нибудь"}, Kill, moder},
		{tele.Command{Text: "kill", Description: "пристрелить кого-нибудь"}, Kill, moder},
		//{tele.Command{Text: "listantispam", Description: "список антиспама"}, checkpoint.ListAntispam, moder},
		{tele.Command{Text: "mute", Description: "заглушить кого-нибудь"}, Mute, moder},
		{tele.Command{Text: "pidordel", Description: "удалить игрока из \"Пидор Дня!\""}, Pidordel, moder},
		{tele.Command{Text: "pidorlist", Description: "список всех игроков \"Пидор Дня!\""}, Pidorlist, moder},
		{tele.Command{Text: "restart", Description: "перезапуск бота"}, Restart, admin},
		{tele.Command{Text: "resurrect", Description: "возродить кого-нибудь"}, Revive, moder},
		{tele.Command{Text: "revive", Description: "возродить кого-нибудь"}, Revive, moder},
		{tele.Command{Text: "addbless", Description: "добавить причину блесса"}, AddBless, moder},
		{tele.Command{Text: "addnope", Description: "добавить сообщение отказа по кнопке"}, AddNope, moder},
		{tele.Command{Text: "ban", Description: "забанить кого-нибудь"}, Ban, moder},
		{tele.Command{Text: "bless", Description: "попросить помолчать кого-нибудь"}, Kill, moder},
		{tele.Command{Text: "debug", Description: "получить сообщение в виде JSON"}, Debug, moder},
		//{tele.Command{Text: "delantispam", Description: "удалить из антиспама"}, checkpoint.DelAntispam, moder},
		{tele.Command{Text: "say", Description: "заставить бота сказать что-нибудь"}, Say, moder},
		{tele.Command{Text: "setgetowner", Description: "задать владельца гета"}, SetGetOwner, moder},
		{tele.Command{Text: "unban", Description: "разбанить кого-нибудь"}, Unban, moder},
		{tele.Command{Text: "unmute", Description: "разглушить кого-нибудь"}, Unmute, moder},
		{tele.Command{Text: "warn", Description: "предупредить кого-нибудь"}, WarnUser, moder},
		//{tele.Command{Text: "testrandom", Description: "протестировать рандом бота "}, TestRandom, moder}
	}

	commandMemberArray := []tele.Command{}
	for i := range commandMemberList {
		Bot.Handle("/"+commandMemberList[i].command.Text, commandMemberList[i].function, commandMemberList[i].middleware)
		commandMemberArray = append(commandMemberArray, commandMemberList[i].command)
	}
	sort.Slice(commandMemberArray, func(i, j int) bool {
		return commandMemberArray[i].Text < commandMemberArray[j].Text
	})
	err := Bot.SetCommands(commandMemberArray, tele.CommandScope{Type: tele.CommandScopeAllGroupChats})
	if err != nil {
		log.Fatal(err)
	}

	commandAdminArray := []tele.Command{}
	for i := range commandAdminList {
		Bot.Handle("/"+commandAdminList[i].command.Text, commandAdminList[i].function, commandAdminList[i].middleware)
		commandAdminArray = append(commandAdminArray, commandAdminList[i].command)
	}
	commandAdminArray = append(commandMemberArray, commandAdminArray...)
	sort.Slice(commandAdminArray, func(i, j int) bool {
		return commandAdminArray[i].Text < commandAdminArray[j].Text
	})
	err = Bot.SetCommands(commandAdminArray, tele.CommandScope{Type: tele.CommandScopeAllChatAdmin})
	if err != nil {
		log.Fatal(err)
	}

	//non-command handles
	Bot.Handle(&AcceptButton, Accept, chats)
	Bot.Handle(&DenyButton, Deny, chats)
	Bot.Handle(tele.OnChatMember, OnChatMember, chats)
	Bot.Handle(tele.OnUserJoined, OnUserJoined, chats)
	Bot.Handle(tele.OnUserLeft, OnUserLeft, chats)
	Bot.Handle(tele.OnText, OnText, chats)
	Bot.Handle(tele.OnQuery, GetInline)
	Bot.Handle(tele.OnChannelPost, ForwardPost)

	Bot.Start()
}
