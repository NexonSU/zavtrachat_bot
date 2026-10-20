# zavtrachat_bot

**Русский** | [English](README.en.md)

Telegram-бот для одного конкретного чата на Go. Работает через Bot API (long polling или webhook,
поддерживается self-hosted Bot API server), дополнительно использует MTProto-клиент для нескольких
команд. Состояние — в PostgreSQL.

- 91 команда, зарегистрированная в `main.go` (часть — алиасы одной функции)
- 94 исходника `.go`, 167 тестов в 25 файлах
- GitHub Actions: сборка + деплой на self-hosted раннере и публикация образа в `ghcr.io`
- Лицензия — MIT
- С 16.10.2026 разработку ведёт ИИ — модели локального стека Strata (`swift-1.5-iq3_xxs`,
  `qwen3.8-flash-next-iq3_xxs` и др., подробности — в разделе [Разработка](#разработка))

## Команды

### Модерация
| Команда | Что делает |
|---|---|
| `/ban`, `/unban` | бан / разбан, опционально время в секундах (`/ban @user 3600`) |
| `/mute`, `/unmute` | заглушить / разглушить, опционально время в секундах |
| `/kick` | кик (бан + разбан, чтобы можно было вернуться по ссылке) |
| `/warn`, `/mywarns` | предупреждение / сколько у меня предупреждений |
| `/remove` | удалить сообщение, ответом на которое послана команда |
| `/debug`, `/debuguser`, `/finduserinmessage` | JSON-дампы сообщения/юзера для отладки |

Доступ к модерационным командам — по спискам `ZAVTRA_ADMINS` / `ZAVTRA_MODERS` / `ZAVTRA_SYSADMIN`.

### Игры и «социальное»
| Команда | Что делает |
|---|---|
| `/duel`, `/russianroulette` | вызов на дуэль (русская рулетка), кнопки принятия/отказа |
| `/duelstats` | своя статистика дуэлей |
| `/kill` и алиасы `bite`, `kilobite`, `kilomlem`, `megabite`, `bless` | «пристрелить» юзера (временное ограничение) |
| `/revive`, `/resurrect`, `/redemption`, `/massrevive`, `/megarevive` | возродить одного / всех недавних |
| `/shotgun`, `gigabite`, `gigakill`, `ultrakill` | выстрел по чату |
| `/megamlem` | те же права и ограничения участников, что у `/shotgun`, с картинкой гигантского чёрного кота |
| `/isekai`, `suicide`, `blessing`, `/isekaitop`, `/blessingtop` | «устроиться в роскомнадзор» и топ чата |
| `/addbless`, `/addnope` | добавить причину блесса / текст отказа по кнопке |
| `/blesslist` | управление общими фразами блесса в личке: страницы, выбор и подтверждение удаления |
| `/pidor`, `/pidoreg`, `/pidorlist`, `/pidorme`, `/pidorstats`, `/pidorall`, `/pidorules`, `/pidordel`, `/pidorremovetoday` | игра «Пидор Дня!» |
| `/slap`, `/hug`, `/bonk`, `/marco`, `/shrug`, `/me` | стикеры/мем-действия и IRC-подобный `/me` |

`/blesslist` доступна в личке пользователям из `ZAVTRA_ADMINS` и `ZAVTRA_MODERS`.
В группе команда только предлагает перейти в личку. Фразы общие для `/blessing`, `/kill`,
`/shotgun`, `/megamlem` и их аналогов. Удаление требует отдельного подтверждения точного текста;
последнюю фразу удалить нельзя. Бот оставляет копию удалённого текста для восстановления через
неизменённую `/addbless` (до 200 символов; для длинных старых фраз выводится предупреждение).
Кнопки привязаны к пользователю и сообщению, действуют 30 минут;
после перезапуска или истечения срока откройте `/blesslist` заново. Статистика не меняется.

### Медиа
| Команда | Что делает | Нужно |
|---|---|---|
| `/meow`, `/mlem`, `/woof` | гифка с котиком / не-котиком | MTProto |
| `/meme` | мем из канала | MTProto |
| `/convert` | конвертация медиа: `mp3`, `ogg`, `gif`, `audio`, `voice`, `animation` | ffmpeg, `ZAVTRA_BOT_API_DATADIR` |
| `/distort`, `/invert`, `/reverse`, `/loop` | «переебать»/инвертировать/залупить медиа | ffmpeg, `ZAVTRA_BOT_API_DATADIR` |
| `/mp3` | скачать аудио по ссылке (YouTube и т. п.) | yt-dlp, ffmpeg |
| `/download`, `/wget` | скачать видео по ссылке | yt-dlp |
| `/savetopm`, `/giveme`, `/topm` | сохранить пост в личку (копирует в PM, бот должен быть разблокирован) | — |

### Утилиты
| Команда | Что делает |
|---|---|
| `/get`, `/getall`, `/set`, `/del`, `/setgetowner` | «геты» — сохраняемые ответы бота (текст/медиа), инлайн-выбор через `@botname` |
| `/sed`, `/khaleesi`, `/say` | замена текста по образцу, мем-кхалиси, заставить бота сказать текст |
| `/stats` | топы чата: `likes`/`reactions` (лайки на сообщения), `mutes`/`blessings` (смерти), `mutetime`/`blesstime` (время в муте), `messages`, `words`; период `day`/`week`, без периода — за всё время |
| `/advice`, `/anekdot`, `/releases` | совет, анекдот, список релизов игр |
| `/cur` | курсы валют (нужен `currency_key`) |
| `/google` | ссылка на поиск по тексту |
| `/date`, `/ping`, `/getid`, `/testrandom`, `/tldr` | дата сервера, понг, ID юзера, проверка рандома, подсказка про кнопку TLDR |
| `/restart` | `os.Exit(0)` — перезапуск бота (нужен `Restart=always` в systemd) |

### Обработчики без команд
- **`reserve_chat`** — удаляет системные сообщения о входе/выходе и добавляет юзеров в БД.
- **`channel`** — пересылает посты канала в чат; при заданном `stream_channel` дублирует туда.
- **Inline-режим** — `@botname <текст>` отдаёт сохранённые геты.
- **Реакции** в основном чате.
- **Глобальный middleware** — debug-лог каждого апдейта + антиспам: больше `ZAVTRA_ANTISPAM_COUNT` сообщений
  за `ZAVTRA_ANTISPAM_WINDOW` секунд → эфемерное предупреждение `ZAVTRA_SYSADMIN` в основном чате.

## Требования

| Зависимость | Где нужна |
|---|---|
| Go | 1.26+ (`go.mod`), в CI/`Dockerfile` база `golang:1.25` + автозамена тулчейна |
| PostgreSQL | обязателен, таблицы создаются `AutoMigrate` при старте |
| ffmpeg, ffprobe | для `/convert`, `/distort`, `/invert`, `/loop`, `/mp3` |
| yt-dlp | для `/mp3`, `/download` (в Docker-образе **не** установлен) |
| Self-hosted Bot API | опционально: `ZAVTRA_BOT_API_URL` (применяется ко всем API-вызовам); `ZAVTRA_BOT_API_DATADIR` — чтобы `GetMedia` читал файлы напрямую с диска (для `/convert`, `/distort`, `/invert`, `/loop`) |
| Прямой доступ к датацентрам Telegram | для MTProto-команд: TCP на `149.154.*` / `91.108.*:443` + `app_id`/`app_hash` |

## Конфигурация

Всё конфигурирование — переменные окружения `ZAVTRA_*`. Бот читает файл `.env` из рабочей директории
(`ZAVTRA_ENV_FILE` указывает на другой путь), а переменные окружения процесса его перекрывают: файл —
это настройки по умолчанию, окружение — точечные переопределения. Шаблон — `.env.example`, сам `.env`
в `.gitignore`: в нём токен бота и пароль базы, держите права `0600`.

Docker: `docker run --env-file .env`, в compose `env_file: .env`, в systemd `EnvironmentFile=`. В образ
`.env` не попадает (`.dockerignore`).

| Переменная | Назначение |
|---|---|
| `ZAVTRA_TOKEN` | токен бота — **обязательна** |
| `ZAVTRA_CHAT` | ID основного чата — **обязательна** |
| `ZAVTRA_DSN` | PostgreSQL DSN |
| `ZAVTRA_BOT_API_URL` | базовый URL Bot API (по умолчанию `https://api.telegram.org`) |
| `ZAVTRA_BOT_API_DATADIR` | datadir self-hosted Bot API для прямого чтения файлов |
| `ZAVTRA_APP_ID`, `ZAVTRA_APP_HASH` | учётные данные MTProto-приложения для `/meow`, `/woof`, `/meme` |
| `ZAVTRA_ALLOWED_UPDATES` | какие апдейты запрашивать (по умолчанию `message,channel_post,callback_query,chat_member`) |
| `ZAVTRA_LISTEN`, `ZAVTRA_ENDPOINT_PUBLIC_URL`, `ZAVTRA_MAX_CONNECTIONS` | webhook-режим; если пусто — long polling |
| `ZAVTRA_RESERVE_CHAT` | чат, где удаляются сообщения о входе/выходе |
| `ZAVTRA_CHANNEL`, `ZAVTRA_STREAM_CHANNEL` | канал для пересылки постов и куда их дублировать |
| `ZAVTRA_ADMINS`, `ZAVTRA_MODERS`, `ZAVTRA_SYSADMIN` | ID с правами модерации (списки через запятую); `ZAVTRA_SYSADMIN` получает уведомления о старте, ошибках и антиспаме эфемерными сообщениями в основном чате (видит только он, Telegram удаляет их по настройке «эфемерные сообщения» чата; нужен self-hosted Bot API) |
| `ZAVTRA_CURRENCY_KEY` | CoinMarketCap API key для `/cur` |
| `ZAVTRA_RELEASES_URL` | календарь для `/releases` (по умолчанию — PS/PS5/Switch/Xbox PAL) |
| `ZAVTRA_PROXY` | HTTP-прокси для исходящих запросов: `/advice`, `/anekdot`, `/releases`, скачивание файлов (таймаут 10 с) |
| `ZAVTRA_ANTISPAM_WINDOW`, `ZAVTRA_ANTISPAM_COUNT` | окно в секундах и порог сообщений для антиспама |
| `ZAVTRA_AI_URL`, `ZAVTRA_AI_TOKEN`, `ZAVTRA_AI_MODEL`, `ZAVTRA_AI_SYSTEM`, `ZAVTRA_AI_ADMIN_ONLY_TOOLS` | AI-модуль (`ai_*.go`), в `main.go` пока выключен |
| `ZAVTRA_FONTPATH`, `ZAVTRA_COMMENT_CHAT` | объявлены в конфиге, в коде сейчас не используются |
| `ZAVTRA_ENV_FILE` | путь к `.env` (по умолчанию `.env` в рабочей директории) |

Списки — через запятую (пробел и `;` тоже разделители). Значения с пробелами, `$`, `#` и переводами
строк берутся в двойные кавычки: `ZAVTRA_AI_SYSTEM="первая строка\nвторая"`.

Права `.env` — `600`: в файле токен бота и пароль базы.

## Запуск

```bash
go build -o zavtrachat_bot
cp .env.example .env && chmod 600 .env   # заполните токен и чат
./zavtrachat_bot            # из каталога, где лежит .env
```

Docker:

```bash
docker build -t zavtrachat .
docker run -d --name zavtrachat --env-file .env zavtrachat
```

В образе `WORKDIR /`, `ENTRYPOINT /goapp`, установлен ffmpeg; конфигурация — `--env-file .env`
(или отдельные `-e ZAVTRA_TOKEN=...`). Секреты в образ не копируются (`.dockerignore`).
`yt-dlp` в образ не входит — `/mp3` и `/download` в контейнере работать не будут.

CI (`.github/workflows/`):
- `deploy.yml` — на изменение `**.go`, `go.mod`/`go.sum` и самого workflow: `Test` гоняет `go vet` и
  `go test -race -count=1` на `ubuntu-latest` (покрытие складывается в артефакт), затем на `push` в
  `master` `Deploy` собирает `telegram-go-chatbot` на self-hosted раннере, копирует его в
  `/home/telegram/bot/` и делает `systemctl restart telegram-go-chatbot`. На pull request тесты
  идут, прод не перезапускается; параллельные деплои блокируются `concurrency`-группой;

  **На сервере в рабочей директории сервиса должен лежать `.env` (`/home/telegram/bot/.env`)** —
  конфигурация читается только из окружения, без файла сервис не стартует. Права `0600`, владелец —
  пользователь сервиса;
- `docker-publish_yml` — сборка и публикация образа в `ghcr.io` + cosign-подпись. **Файл назван с
  `_yml` вместо `.yml`, поэтому GitHub Actions его не подхватывает** (workflows ищут `*.yml`/`*.yaml`).

## Тесты

```bash
go test -race -count=1 ./...                                   # 167 тестов
PROTO_PROBE=1 go test -count=1 -run TestMtprotoConnectProbe -v .   # проверить доступ к датацентрам
```

Инфраструктура тестов — `testutils_test.go`:
- фейковый Bot API-клиент, записывающий все вызовы бота (проверяется отправленный текст/действия);
  хук `Observe` даёт заглянуть в файл, который обработчик закрывает и удаляет сразу после отправки;
- in-memory SQLite (`github.com/glebarez/sqlite`, чистый Go, CGO не нужен) вместо PostgreSQL;
- хелперы `setupTestEnv`, `testMessage`, `testContext`, `testCallbackContext`, `seedUser`,
  `waitForCalls`.

Сети и Postgres тесты не требуют. Учитывайте: тесты идут на SQLite, прод — на PostgreSQL, поэтому
поведение БД в проде тестами не доказывается (`TestDataBaseInit` проверяет только пути отказа).
Зависимости от окружения: четырнадцать тестов гоняют настоящие `ffmpeg`/`ffprobe` по сгенерированным
секундным файлам и пропускаются, если бинарников нет (`TestFFmpegConvert`,
`TestFFmpegConvertConcurrentSameName`, `TestConvert`, `TestInvert`, `TestDistort`,
`TestLoopAnimation`, `TestDownloadFile`, `TestFFmpegRemoteMedia`, `TestGetImageForLLM`,
`TestGetImageForLLMRemote`, `TestGetImageForLLMConcurrentSameFile`, `TestAIWithPhoto`,
`TestMp3LocalSource`, `TestDownloadLocalSource`); `TestMtprotoConnectProbe` пропускается без
`PROTO_PROBE=1`, `TestYtdlpGetVer` — без установленного `yt-dlp`.

Временные файлы конвертаций (`/convert`, `/mp3`, `/download`, картинка для `/ai`) и рабочие папки
`/distort` получают уникальные имена — `os.CreateTemp`, `chat_id` + `message_id`, `DistortTempDir`
для тестов. Поэтому несколько прогонов набора на одной машине не перетирают друг другу файлы, а
параллельные запросы на один и тот же файл не читают чужую копию; это проверяют
`TestFFmpegConvertConcurrentSameName` и `TestGetImageForLLMConcurrentSameFile`.

`/mp3` и `/download` работают с реальным `yt-dlp` на локальных медиафикстурах: `ytdlpFixtureServer`
генерирует секундный m4a-файл и HLS-поток с отдельными видео- и аудиодорожками, отдаёт их по HTTP на
`127.0.0.1`, а `TestMain` выписывает loopback из прокси (`NO_PROXY`), чтобы `yt-dlp` шёл к фикстуре
напрямую. `TestMp3LocalSource` и `TestDownloadLocalSource` проверяют, что фильтр форматов выбранного
качества действительно находит фикстуру, `ffmpeg` склеивает/конвертирует дорожки, а отправленный файл
— это готовый mp3 и готовый mp4 с обеими дорожками.

Наружу тесты не ходят: весь внешний трафик уходит в локальные заглушки. `mitm_test.go` поднимает
MITM-прокси — `TestMain` генерирует одноразовый CA, пишет его в `SSL_CERT_FILE` и публикует прокси
через `HTTP_PROXY`/`HTTPS_PROXY`, поэтому в заглушку попадают даже клиенты с дефолтным транспортом
(клиент CoinMarketCap); сертификаты для хостов подписываются на лету. Два пути перехвата:
`stubTLSProxy` (прокси для `httpClient()`) и `stubEnvRoute` (маршрут поверх прокси процесса).
Локальные `httptest`-серверы: заглушка Bot API (`TestBotInit` — `getMe`, удаление вебхука, polling
и отчёт сисадмину; `TestBotInitWebhook` — сервер вебхука поднимается до `setWebhook`, запрос без
secret-токена получает 401), OpenAI-формат для `/ai` (`TestAIStream`, `TestAIStreamError`,
`TestAIWithPhoto`). Особые приёмы: `TestRestart` запускает процесс заново (`os.Exit` внутри
обработчика нельзя проверить из того же процесса), `TestConfigInit` работает в `t.TempDir()` и с чистым
окружением (`clearZavtraEnv`), чтобы не трогать `.env` чекаута. `TestStartupReport` проверяет отчёт о
старте на секреты: в нём нет токена, а из DSN, прокси, app hash и API-ключей вырезаны пароли.

## Структура

```
main.go              конфигурация обработчиков и запуск
utils_bot.go         инициализация бота, polling/webhook, middleware+антиспам
utils_bot_proto.go   MTProto-клиент: подключение, переподключение, таймауты
utils_config.go      Configuration + переменные окружения ZAVTRA_* (.env)
utils_db.go          модели gorm, AutoMigrate, текстовые пулы «да/нет»
utils_handlers.go    обработчики апдейтов: апдейт юзера, альбом-кэш, проверка банов, реакции
utils_user.go        имена и меншены юзеров, списки админов, поиск юзера в сообщении и в БД
utils_stats.go       счётчики статистики сообщений, слов и времени в муте, начало суток
utils_reply.go       Reply, ReplyAndRemove и SysAdminReply — чем команды отвечают пользователю и сисадмину
utils_startup.go     отчёт о старте: сборка, хост, соединение, БД, чаты, версии инструментов (без кредов)
utils_common.go      RandInt, StringInSlice, MarshalOmitEmptyAll
utils_http.go        HTTP-клиент с прокси и таймаутом, DownloadFile
utils_forward.go     пересылка постов канала
utils_files.go       go:embed стикеров и картинки /megamlem (assets/*.png, assets/megamlem.jpg)
utils_plural.go      русские числительные (ruPlural)
media.go             медиа сообщения: тип, файл, где он лежит на диске
media_ffmpeg.go      FFmpegConvert — конвертация для /convert, /distort, /invert, /loop
command_*.go         команды
duel_*.go            дуэль: request/accept/stats
pidor_*.go           игра «Пидор Дня!»
bets_*.go            ставки
ai_*.go              AI-модуль (выключен)
*_test.go            тесты + harness
assets/              стикеры, картинка /megamlem (go:embed) и CREDITS.md
```

## MTProto-клиент

`/meow`, `/woof` и `/meme` работают через MTProto (`gotgproto` + `gotd`), а не через Bot API:

- клиент поднимается в фоновой горутине, подключение ограничено `protoConnectTimeout` (45 с),
  при неудаче — повтор каждые `protoReconnectDelay` (30 с); в логе видно `MTProto client connected`
  или причину (`MTProto client is not connected (attempt=N, error=…)`);
- команды при неподключённом клиенте отвечают «MTProto-клиент ещё не подключился», а не падают;
- на каждый запрос свой дедлайн (RPC 60 с, скачивание 3 мин), read-only запросы ретраятся;
- `NoUpdates: true` — бот не забирает апдейты у long polling Bot API, MTProto используется только
  для RPC;
- `ZAVTRA_PROXY` на MTProto **не влияет**: датацентры диалсятся прямым TCP, поэтому для этих команд
  нужен прямой доступ к `*:443` (или self-hosted MTProto-транзит).

## Известные особенности

- `ZAVTRA_PROXY` применяется только к обычным HTTP-запросам бота (advice/anekdot/releases/скачивание
  файлов); вызовы Bot API и MTProto его не используют.
- `/restart` делает `os.Exit(0)` — перезапуск должен обеспечивать внешний супервизор (`Restart=always`
  в systemd, `restart: always` в compose).
- Ошибки хендлеров диспетчер отправляет в чат («Ошибка: …» и контекст: текст команды, юзер, чат), токен в тексте подменяется на `TOKEN`.
- AI-команды (`/ai`, `/restartai`, `/setaisystem`) закомментированы в `main.go`.

## Разработка

С **16 октября 2026 года** разработкой занимается ИИ: правки пишет модель, человек ставит задачи,
принимает решения и проверяет результат. Каждый такой коммит помечен в сообщении
(`Made with the … model.`) и соавтором (`Co-authored-by: … <noreply@opencode.ai>`), поэтому вклад ИИ
виден прямо в истории git.

Модели локального стека Strata — конфигурации в `~/.strata/*.json`, движок `~/.strata/engine/strata`,
веса в `~/Strata-data/models`:

| Модель (`model_name`) | Что это | В этом репозитории |
|---|---|---|
| `swift-1.5-iq3_xxs` | Swift-1.5 Qwen3.8-Flash-Next GSQ RCO, квант IQ3_XXS (`ukisai/Swift-1.5-Qwen3.8-Flash-Next-GSQ-RCO-GGUF`) | текущая: тест-харнес, фиксы MTProto/числительных, тесты команд, фиксы старта бота и `/ai`, тесты меню команд, тесты `/mp3`/`/download` на реальном `yt-dlp`, конфигурация через `.env`, README |
| `qwen3.8-flash-next-iq3_xxs` | Qwen3.8-Flash-Next GSQ RCO, квант IQ3_XXS (`ISTA-DASLab/Qwen3.8-Flash-Next-GSQ-RCO-GGUF`) | `e55e505` (go:embed), `30415bd` (тесты хелперов + фикс `RandInt`) |
| `qwen3.8-flash-next` | она же, до отдельного кванта | `4ab27c5` — первый ИИ-коммит (`bot_api_url` для всех API-вызовов) |
| `orcarouter-qwen3.8-flash-next-uncensored-iq3_xxs` | Qwen3.8-Flash-Next Uncensored, IQ3_XXS | настроена в `~/.strata`, к этому коду не применялась |

Рядом с ними в стеке: vision-проекции `mmproj-Qwen3.8-Flash-Next-BF16.gguf` и
`mmproj-Swift-Qwen3.8-Flash-Next-BF16.gguf`, черновик для спекулятивного декодирования
`mtp-q2_0.gguf`, а в `~/.strata/data/gguf_fingerprints.json` описаны и другие кванты этих же моделей
(Q2_0, IQ2_XS, IQ3_XXS, IQ3_S, coder IQ1_M, unsloth UD-IQ4_XS / UD-Q4_K_XL).

## Лицензия

MIT (см. `LICENSE`), © 2026 NexonSU. Лицензия покрывает исходники бота.

Что она не покрывает:

- `assets/bonk.png` и `assets/hug.png` — сторонние мемы, права на них не передаются; подробности в
  `assets/CREDITS.md`. При публичной публикации их стоит убрать или заменить на свои.
- `ffmpeg` и `yt-dlp` — внешние GPL-программы, бот вызывает их отдельным процессом; лицензия бота
  на них не распространяется, условия их лицензий — на их стороне.
- Почти все Go-зависимости — MIT / BSD-3 / Apache-2.0, они с MIT совместимы. Исключение —
  `github.com/celestix/gotgproto` под **GPL-3.0**: он линкуется в бинарник, поэтому при
  распространении собранного бота условия GPL-3 действуют на работу целиком (придётся раскрыть
  исходники). Для запуска бота на своём сервере обязательств нет: GPL срабатывает при
  распространении, а не при использовании.
