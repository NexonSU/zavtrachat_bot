package main

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

const zbHelpText = `
/quote — публичная цитата: общий лимит участников 5 в час, админы без лимита.
<b>ZavtraBot — команды участников</b>
Эту справку видишь только ты. «Ответом» — отправь команду в ответ на нужное сообщение.

<b>Геты — сохранённые тексты и медиа</b>
/get название — получить гет.
/getall — список гетов; поиск: @имя_бота текст.
/set название текст — создать гет; ответом на текст или медиа: /set название. Менять существующий гет может его автор или админ.
/del название — удалить свой гет.

<b>Игры и статистика</b>
/duel, /russianroulette — вызвать на дуэль ответом или указав @ник; второй участник принимает вызов кнопкой.
/duelstats — своя статистика дуэлей.
/blessing, /suicide, /isekai — самоблесс: возможны спасение или игровой мут; у админов возможен рикошет.
/blessingtop, /isekaitop — топ блессов.
/pidoreg — записаться в «Пидор Дня».
/pidor — розыгрыш дня; /pidorules — правила.
/pidorme — своя статистика; /pidorstats — за год; /pidorall — за всё время.
/stats вид [day|week] — топ: messages, words, likes (reactions), mutes (blessings), mutetime (blesstime), saves. Без периода — за всё время. Пример: <code>/stats messages week</code>.
/mywarns — свои предупреждения.
/bet ДД.ММ.ГГГГ текст — сохранить ставку на будущую дату.
/allbets — актуальные ставки; аргумент all — включая старые.
/delbet ДД.ММ.ГГГГ текст — удалить свою ставку.

<b>Медиа</b>
/meow, /mlem — гифка с котиком; /woof — с не-котиком; /meme — мем.
/download, /wget ссылка — скачать видео; /mp3 ссылка — скачать аудио. Можно ответить на сообщение со ссылкой.
/convert формат — ответом на медиа: mp3, ogg, gif, audio, voice, animation.
/invert, /reverse — воспроизвести медиа задом наперёд, ответом на него.
/loop — зациклить гифку; /distort — исказить медиа, ответом на него.
/savetopm, /giveme, /topm — скопировать сообщение в личку, ответом на него. Сначала запусти бота в личке.

<b>Действия и полезное</b>
/hug, /bonk — обнять или бонкнуть ответом на сообщение; /slap — дать леща ответом или с @ником.
/me текст — действие от своего имени; /shrug — пожать плечами; /marco — «Поло».
/sed s/старое/новое/ — заменить текст в копии сообщения, ответом на него.
/khaleesi — мемно преобразовать текст, ответом на сообщение.
/advice — совет; /anekdot — анекдот; /releases — ближайшие релизы игр.
/cur — курсы валют; /google текст — ссылка на поиск.
/tldr — подсказка о кнопке пересказа Telegram.
/date — дата сервера; /ping — проверка ответа бота.
/zbhelp — эта справка.`

const zbAdminHelpText = `<b>ZavtraBot — команды администраторов</b>
Эту справку видишь только ты. Для большинства команд нужны права в списках админов/модераторов бота; одного статуса админа Telegram недостаточно. Справка не расширяет права.
Участник задаётся ответом на его сообщение, @ником или ID, если ниже не указано иначе.

<b>Модерация</b>
/mute — запретить писать; /unmute — снять мут.
/ban — забанить; /unban — разбанить; /kick — исключить с возможностью вернуться.
Время для /mute и /ban — в секундах, по умолчанию сутки. Пример: <code>/mute @user 3600</code>; ответом: <code>/mute 3600</code>.
/warn — выдать предупреждение; третье приводит к муту на неделю.
/remove — удалить сообщение, на которое отвечаешь.
/muted — кто сейчас замьючен, кем и почему, остаток времени. Только в основном чате; доступ также у админов/владельца Telegram и системного администратора.

<b>Игровые ограничения и возрождения</b>
/kill — пристрелить; /bless — попросить помолчать; /bite — кусь.
/kilobite, /kilomlem, /megabite — усиленные варианты с увеличенным сроком мута. При цели-админе возможен рикошет.
/shotgun, /gigabite, /gigakill, /ultrakill — массовый игровой выстрел по недавним участникам.
/megamlem — массовый МЕГАМЛЕМ с чёрным котом.
/revive, /resurrect — возродить участника.
/redemption, /massrevive, /megarevive — возродить недавних участников с ограничениями.

<b>Игра «Пидор Дня»</b>
/pidorlist — список зарегистрированных игроков.
/pidordel — удалить участника из игры.
/pidorremovetoday — сбросить сегодняшний результат розыгрыша.

<b>Фразы и геты</b>
/addbless текст — добавить причину блесса (до 200 символов).
/addsalvation текст — добавить фразу спасения (до 200 символов).
/addnope текст — добавить фразу отказа.
Добавлять фразы можно также командой в ответ на текст.
/blesslist — просмотр и удаление фраз блесса с подтверждением.
/blesslistfull — все фразы блесса текстом.
/salvationlist — просмотр и удаление фраз спасения с подтверждением.
/salvationlistfull — все фразы спасения текстом.
Просмотр списков фраз работает в личке с ботом.
/setgetowner название — передать гет автору сообщения, на которое отвечаешь. Админы также могут менять и удалять чужие геты.

<b>Служебные</b>
/say текст — отправить текст от имени бота.
/getid — сведения о пользователе (в личку).
/debug — JSON сообщения (в личку); /debuguser — сведения об участнике (в личку).
/finduserinmessage — найти участника в сообщении, на которое отвечаешь.
/testrandom — распределение случайных чисел.
/perf [15|60] — производительность, память и очередь медиа; только в личке.
/restart — перезапустить бота.
/zbadminhelp — эта справка.`

func ZBHelp(bot *gotgbot.Bot, ctx *ext.Context) error {
	text := zbHelpText
	if bot.Username != "" {
		text = strings.ReplaceAll(text, "@имя_бота", "@"+bot.Username)
	}
	return sendZBHelp(bot, ctx, text)
}

func ZBAdminHelp(bot *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveUser == nil || ctx.EffectiveMessage == nil {
		return nil
	}
	if !canReadMuted(bot, ctx.EffectiveUser.Id) {
		return sendZBHelp(bot, ctx, "Административная справка доступна только администраторам и модераторам чата.")
	}
	return sendZBHelp(bot, ctx, zbAdminHelpText)
}

// A group response, including access denials, is always recipient-specific.
// Delivery errors propagate without retrying as a public message.
func sendZBHelp(bot *gotgbot.Bot, ctx *ext.Context, text string) error {
	if ctx.EffectiveChat == nil || ctx.EffectiveUser == nil || ctx.EffectiveMessage == nil {
		return nil
	}
	opts := &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML}
	switch ctx.EffectiveChat.Type {
	case "private":
	case "group", "supergroup":
		opts.EphemeralMessageParameters = &gotgbot.EphemeralMessageParameters{ReceiverUserId: ctx.EffectiveUser.Id}
	default:
		return nil
	}
	_, err := bot.SendMessage(ctx.EffectiveChat.Id, text, opts)
	return err
}
