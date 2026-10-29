# zavtrachat_bot

[Русский](README.md) | **English**

A Go Telegram bot for one specific chat. It works through the Bot API (long polling or webhook,
self-hosted Bot API server supported) and additionally uses an MTProto client for a few commands.
State is kept in PostgreSQL.

- 99 commands registered in `main.go` (some of them are aliases of the same function)
- 112 `.go` source files, 291 tests in 49 files
- GitHub Actions: build + deploy on a self-hosted runner, image publishing to `ghcr.io`
- License — MIT
- Since 16.10.2026 development is driven by AI — models of the local Strata stack (`swift-1.5-iq3_xxs`,
  `qwen3.8-flash-next-iq3_xxs`, etc., details in the [Development](#development) section)

## Commands

### Help

- `/zbhelp` — participant commands with descriptions, examples and aliases (in Russian).
- `/zbadminhelp` — administrative commands; available to main-chat Telegram administrators/owner and configured admins, moderators and sysadmin.

Both group commands and their responses are ephemeral and visible only to the caller,
including access denials. Private chats receive ordinary private replies. Failed ephemeral
delivery never falls back to a public response. `/zbhelp` appears in group and private menus;
`/zbadminhelp` appears in the main-chat administrator menu and can also be typed manually
in private chats. Existing command permissions are unchanged.

### Moderation
| Command | What it does |
|---|---|
| `/ban`, `/unban` | ban / unban, optional duration in seconds (`/ban @user 3600`) |
| `/mute`, `/unmute` | mute / unmute, optional duration in seconds |
| `/muted` | current mutes, origin, initiator and time remaining; an ephemeral response to the requesting admin |
| `/kick` | kick (ban + unban, so the user can come back through an invite link) |
| `/warn`, `/mywarns` | warn somebody / how many warnings I have |
| `/remove` | delete the message the command was sent in reply to |
| `/debug`, `/debuguser`, `/finduserinmessage` | JSON dumps of a message/user for debugging |

Access to moderation commands comes from the `ZAVTRA_ADMINS` / `ZAVTRA_MODERS` / `ZAVTRA_SYSADMIN` lists.

`/muted` runs in the main chat. Telegram administrators/owner and configured
`ZAVTRA_ADMINS`, `ZAVTRA_MODERS`, `ZAVTRA_SYSADMIN` can use it. The command is advertised
in the chat administrator menu; configured moderators can invoke it by name.
Both the menu command and its response are ephemeral. Refresh/pagination edits only
the requester's ephemeral message and rechecks permissions. End times use Moscow time.

The full live list is fetched over MTProto, including manual and pre-existing restrictions.
If MTProto is unavailable, Bot API checks the bot's known active restrictions and the report
explicitly warns that it is partial; failed checks are counted separately.
Lifted/expired restrictions, departed users and media-only restrictions are excluded.
The new `mute_events` table records successful restrictions/lifts, initiator and origin:
self-bless, ricochet, kill, duel, shotgun, admin mute or three warnings. Manual changes
are observed via `chat_member` (the bot must be an administrator and `chat_member` must
be in `ZAVTRA_ALLOWED_UPDATES`, as it is by default). Missing/mismatched origins are
shown as unknown. Existing game counters and restriction rules are unchanged.

### Games and "social"
| Command | What it does |
|---|---|
| `/duel`, `/russianroulette` | challenge somebody to a duel (Russian roulette), accept/deny buttons |
| `/duelstats` | your own duel statistics |
| `/kill` and aliases `bite`, `kilobite`, `kilomlem`, `megabite`, `bless` | "shoot" a user (temporary restriction) |
| `/revive`, `/resurrect`, `/redemption`, `/massrevive`, `/megarevive` | revive one user / all recent ones |
| `/shotgun`, `gigabite`, `gigakill`, `ultrakill` | shoot the whole chat |
| `/megamlem` | the same permissions and restrictions as `/shotgun`, with a giant black cat picture |
| `/isekai`, `suicide`, `blessing`, `/isekaitop`, `/blessingtop` | "get hired by Roskomnadzor" and the chat leaderboard |
| `/addbless`, `/addnope` | add a blessing reason / a text shown when the duel button is declined |
| `/addsalvation` | add a salvation phrase (up to 200 characters), as command text or a reply |
| `/salvationlist` | manage the separate salvation phrase pool in PM, with pages and confirmed deletion |
| `/salvationlistfull` | show every salvation phrase as plain text in PM |
| `/blesslist` | manage the shared blessing phrases in PM: pages, selection and deletion confirmation |
| `/pidor`, `/pidoreg`, `/pidorlist`, `/pidorme`, `/pidorstats`, `/pidorall`, `/pidorules`, `/pidordel`, `/pidorremovetoday` | the "Pidor of the Day!" game |
| `/slap`, `/hug`, `/bonk`, `/marco`, `/shrug`, `/me` | stickers/meme actions and the IRC-style `/me` |

`/blesslist` is available in PM to users in `ZAVTRA_ADMINS` and `ZAVTRA_MODERS`.
In groups it only directs users to PM. The pool is shared by `/blessing`, `/kill`, `/shotgun`,
`/megamlem` and their aliases. Deletion requires a separate confirmation of the exact text;
the last phrase cannot be deleted. The bot keeps a recovery copy of the deleted text, which can
be restored through the unchanged `/addbless` (up to 200 characters; longer legacy phrases show
a warning). Buttons are bound to the user and message and
expire after 30 minutes; reopen `/blesslist` after expiry or a restart. Statistics are unchanged.

### Media
| Command | What it does | Needs |
|---|---|---|
| `/meow`, `/mlem`, `/woof` | a gif of a cat / a not-cat | MTProto |
| `/meme` | a meme from a channel | MTProto |
| `/quote` | a quote card from the replied-to message | — |
| `/convert` | media conversion: `mp3`, `ogg`, `gif`, `audio`, `voice`, `animation` | ffmpeg, `ZAVTRA_BOT_API_DATADIR` |
| `/distort`, `/invert`, `/reverse`, `/loop` | mangle / invert / loop media | ffmpeg, `ZAVTRA_BOT_API_DATADIR` |
| `/mp3` | download audio from a link (YouTube etc.) | yt-dlp, ffmpeg |
| `/download`, `/wget` | download video from a link | yt-dlp |
| `/savetopm`, `/giveme`, `/topm` | save a post to PM (copies it into the private chat, the bot must be unblocked) | — |

`/quote` is restricted to configured bot administrators, the system administrator, and Telegram administrators/owner of the main chat. Moderators without administrator rights and regular members are denied. It appears only in administrator menus/help. Reply with `/quote` to text or a media caption. The bot creates a gradient PNG card with
a circular avatar, original text, author, and original timestamp in Moscow time.
Forwarded messages retain their original attribution, including Telegram's hidden-author
name. Missing avatars fall back to initials. Fonts are embedded; ordinary emoji are
monochrome. Each new card independently selects one of four embedded Telegram-inspired wallpapers: light, night, lilac or mint. Every wallpaper is 1200×660 px. Night uses white ink and light themes use dark ink. Selection is independent of the author; repeats are allowed. Normal cards are 1200×660 px; long text expands the height, with the background covering the canvas without changing its aspect ratio. Command arguments cannot replace
the quotation. Text wraps without truncation; messages exceeding a 6000 px card height
receive a usage hint instead. Hints and the card are visible only to the invoking administrator. Group photos use recipient-specific ephemeral parameters; PM responses stay private. Delivery errors never trigger a public fallback. Rendering uses the bounded media queue. No new configuration,
external generation services or database migration are required.

### Utilities
| Command | What it does |
|---|---|
| `/get`, `/getall`, `/set`, `/del`, `/setgetowner` | "gets" — saved bot answers (text/media), inline selection via `@botname` |
| `/sed`, `/khaleesi`, `/say` | pattern-based text replacement, meme-khaleesi, making the bot say a text |
| `/stats` | chat tops: `likes`/`reactions` (likes per message), `mutes`/`blessings` (deaths), `mutetime`/`blesstime` (time muted), `messages`, `words`; period `day`/`week`, no period means all time |
| `/advice`, `/anekdot`, `/releases` | an advice, a joke, a list of game releases |
| `/cur` | currency rates (needs `currency_key`) |
| `/google` | a search link for the given text |
| `/date`, `/ping`, `/getid`, `/testrandom`, `/tldr` | server date, pong, user ID, randomness check, note about the TLDR button |
| `/restart` | `os.Exit(0)` — restarts the bot (needs `Restart=always` in systemd) |

### Performance measurement

`/perf` reports performance in PM to users in `ZAVTRA_ADMINS` and `ZAVTRA_MODERS`.
The default report includes lifetime totals and compact 15/60-minute summaries.
`/perf 15` and `/perf 60` select detailed recent reports. Windows include the current
partial minute and the previous 14/59 complete minutes, attributed at completion.
Only 60 minute aggregates are retained; no event history is stored.

Reports also show Go allocated/system memory, goroutine count, and live active/waiting
media jobs with capacity limits. Go memory is not process RSS and excludes separate
FFmpeg/yt-dlp processes. Queue counts are current, regardless of the selected window.
Group usage hints are visible only to the requesting user.

Group invocations point to PM; other users cannot access the report. Since startup,
metrics track completed handler calls, returned errors, calls taking at least 5 seconds,
and mean/max duration. The report lists up to 10 handlers by cumulative time; command
aliases are separate. All queries through the main GORM client are also counted, with
errors, mean/max duration and queries taking at least 200 ms, including startup and
background queries. Record-not-found results and dispatcher flow signals are not errors.

Only aggregates keyed by fixed handler names are retained, without individual messages,
arguments or user IDs. A restart resets them; no extra database or migration is needed.
Handler duration includes synchronous Telegram calls but excludes goroutines after return.
Internally handled errors do not count as handler failures; SQL failures count separately.
This does not measure the update queue or end-to-end delivery time. Driver BEGIN/COMMIT,
MTProto storage and third-party SQL clients are outside the GORM counter.

Slow or failed handlers and queries produce duration/failure events in the existing log.
The new SQL logger never includes SQL, parameters or error text. Existing diagnostics in
other parts of the bot are unchanged. The server owner can read these events using
`journalctl -u telegram-go-chatbot --since today | grep -E 'handler performance|database performance'`.

### Handlers without commands
- **`reserve_chat`** — deletes join/leave system messages and adds users to the DB.
- **`channel`** — forwards channel posts into the chat; with `stream_channel` set, duplicates them there.
- **Inline mode** — `@botname <text>` returns the saved gets.
- **Reactions** in the main chat.
- **Global middleware** — debug log of every update + anti-spam: more than `ZAVTRA_ANTISPAM_COUNT` messages
  within `ZAVTRA_ANTISPAM_WINDOW` seconds → an ephemeral warning for `ZAVTRA_SYSADMIN` in the main chat.

## Requirements

| Dependency | Where it is needed |
|---|---|
| Go | 1.26+ (`go.mod`); CI/`Dockerfile` use `golang:1.25` as the base with toolchain auto-switch |
| PostgreSQL | required, tables are created by `AutoMigrate` at startup |
| ffmpeg, ffprobe | for `/convert`, `/distort`, `/invert`, `/loop`, `/mp3` |
| yt-dlp | for `/mp3`, `/download` (**not** installed in the Docker image) |
| Self-hosted Bot API | optional: `ZAVTRA_BOT_API_URL` (applied to every API call); `ZAVTRA_BOT_API_DATADIR` so `GetMedia` reads files straight from disk (for `/convert`, `/distort`, `/invert`, `/loop`) |
| Direct access to Telegram datacenters | for the MTProto commands: TCP to `149.154.*` / `91.108.*:443` plus `app_id`/`app_hash` |

## Configuration

Everything is configured through `ZAVTRA_*` environment variables. The bot loads a `.env` file from its
working directory (`ZAVTRA_ENV_FILE` points somewhere else), and the process environment overrides it:
the file holds the defaults, the environment holds the overrides. The template is `.env.example`; the
`.env` itself is in `.gitignore` — it carries the bot token and the database password, keep it `0600`.

Docker: `docker run --env-file .env`, compose `env_file: .env`, systemd `EnvironmentFile=`. The `.env`
never reaches the image (`.dockerignore`).

| Variable | Purpose |
|---|---|
| `ZAVTRA_TOKEN` | bot token — **required** |
| `ZAVTRA_CHAT` | ID of the main chat — **required** |
| `ZAVTRA_DSN` | PostgreSQL DSN |
| `ZAVTRA_BOT_API_URL` | Bot API base URL (defaults to `https://api.telegram.org`) |
| `ZAVTRA_BOT_API_DATADIR` | datadir of a self-hosted Bot API for reading files directly |
| `ZAVTRA_APP_ID`, `ZAVTRA_APP_HASH` | credentials of the MTProto app, used by `/meow`, `/woof`, `/meme` |
| `ZAVTRA_ALLOWED_UPDATES` | which updates to request (defaults to `message,channel_post,callback_query,chat_member`) |
| `ZAVTRA_LISTEN`, `ZAVTRA_ENDPOINT_PUBLIC_URL`, `ZAVTRA_MAX_CONNECTIONS` | webhook mode; if empty — long polling |
| `ZAVTRA_RESERVE_CHAT` | chat where join/leave messages are deleted |
| `ZAVTRA_CHANNEL`, `ZAVTRA_STREAM_CHANNEL` | channel to forward posts from, and where to duplicate them |
| `ZAVTRA_ADMINS`, `ZAVTRA_MODERS`, `ZAVTRA_SYSADMIN` | IDs with moderation rights (comma separated lists); `ZAVTRA_SYSADMIN` gets start, error and anti-spam notifications as ephemeral messages in the main chat (only they can read them, Telegram destroys them by the chat's ephemeral message setting; needs a self-hosted Bot API) |
| `ZAVTRA_CURRENCY_KEY` | CoinMarketCap API key for `/cur` |
| `ZAVTRA_RELEASES_URL` | calendar used by `/releases` (defaults to a PS/PS5/Switch/Xbox PAL calendar) |
| `ZAVTRA_PROXY` | HTTP proxy for outgoing requests: `/advice`, `/anekdot`, `/releases`, file downloads (10 s timeout) |
| `ZAVTRA_ANTISPAM_WINDOW`, `ZAVTRA_ANTISPAM_COUNT` | window in seconds and message threshold for anti-spam |
| `ZAVTRA_AI_URL`, `ZAVTRA_AI_TOKEN`, `ZAVTRA_AI_MODEL`, `ZAVTRA_AI_SYSTEM`, `ZAVTRA_AI_ADMIN_ONLY_TOOLS` | AI module (`ai_*.go`), currently disabled in `main.go` |
| `ZAVTRA_FONTPATH`, `ZAVTRA_COMMENT_CHAT` | declared in the config, currently unused in code |
| `ZAVTRA_ENV_FILE` | path of the `.env` file (defaults to `.env` in the working directory) |

Lists are comma separated (spaces and `;` separate as well). Values with spaces, `$`, `#` or newlines go
in double quotes: `ZAVTRA_AI_SYSTEM="first line\nsecond line"`.

The `.env` file is `600`: it holds the bot token and the database password.

## Running

```bash
go build -o zavtrachat_bot
cp .env.example .env && chmod 600 .env   # fill in the token and the chat
./zavtrachat_bot            # from the directory that holds .env
```

Docker:

```bash
docker build -t zavtrachat .
docker run -d --name zavtrachat --env-file .env zavtrachat
```

The image has `WORKDIR /`, `ENTRYPOINT /goapp` and ffmpeg installed; configuration comes from
`--env-file .env` (or single `-e ZAVTRA_TOKEN=...`). Secrets are never copied into the image
(`.dockerignore`). `yt-dlp` is not part of the image — `/mp3` and `/download` will not work in the container.

CI (`.github/workflows/`):
- `deploy.yml` — on `**.go`, `go.mod`/`go.sum` and workflow changes: `Test` runs `go vet` and
  `go test -race -count=1` on `ubuntu-latest` with a separate PostgreSQL 18 service
  (the coverage profile is uploaded as an artifact), then on
  `push` to `master` `Deploy` builds `telegram-go-chatbot` on a self-hosted runner, copies it to
  `/home/telegram/bot/` and runs `systemctl restart telegram-go-chatbot`. Pull requests run the tests
  only and never restart production; `concurrency` blocks overlapping deploys;

  **The server needs a `.env` in the working directory of the service (`/home/telegram/bot/.env`)** —
  the configuration is read from the environment only, so without the file the service will not start.
  `0600`, owned by the service user;
- `docker-publish_yml` — builds the image and publishes it to `ghcr.io` with a cosign signature.
  **The file is named `_yml` instead of `.yml`, so GitHub Actions never picks it up** (workflows are
  looked for as `*.yml`/`*.yaml`).

## Tests

```bash
go test -race -count=1 ./...                                   # PostgreSQL tests optional locally
PROTO_PROBE=1 go test -count=1 -run TestMtprotoConnectProbe -v .   # check datacenter access
```

CI installs ffmpeg, ffprobe, ImageMagick (`convert`) and yt-dlp `2026.07.04`.
With `ZAVTRA_REQUIRE_MEDIA_TESTS=1`, missing tools or unusable media fixtures fail
instead of skipping. The summary reports skipped tests and permits only the opt-in
live MTProto probe. The `coverage` artifact includes both the coverage profile and
`test-results.jsonl`. Media checks remain optional locally without these tools.

Duel tests control randomness and pauses while preserving production probabilities
and animation delays. They cover concurrent/repeated buttons, stale invitations,
ricochets, draws and Telegram errors. Invitations bind to their message and players;
completed buttons cannot restart a game, and pending invitations expire on restart.

Cat/meme tests replace the MTProto wire transport while exercising real response
decoding and file downloading. They assert sent media bytes/types, empty/error
responses, missing filenames and concurrent channel-cache access without credentials
or live Telegram channels.

Command scenarios cover concurrent warnings/game deaths, database write failures,
Telegram rejection and failed final delivery. Deaths retain their game-attempt meaning:
lifetime and daily counters advance in one transaction before the Telegram call;
mute time advances only after a confirmed restriction. Counter errors are propagated.
A delivery failure does not undo a successful mute. A rejected third warning is rolled
back so a new command can retry; an already reached threshold does not repeat the mute.
New `/kill` and `/blessing` commands remain new game actions. There is no general
exactly-once guarantee across Bot API calls and database commits.

`routing_scenarios_test.go` uses production middleware/handler registration for normal
and ephemeral commands, bot addressing, permissions, callbacks and `chat_member`.
`remaining_postgres_test.go` covers fresh/repeated migration, an older schema, reconnect
reads and concurrent game commands in isolated schemas. Further tests cover all six get
media types, complete formatted captions, ownership, malformed input, weekly warning
boundaries and daily draw midnight/DST. `FuzzSedPattern` includes a seed corpus; run
`go test -run '^$' -fuzz '^FuzzSedPattern$' -fuzztime 20s .` for generated input.

The test infrastructure is `testutils_test.go`:
- a fake Bot API client that records every call the bot makes (the sent text/actions are asserted);
  the `Observe` hook lets a test look inside a file the handler closes and removes right after sending;
- in-memory SQLite (`github.com/glebarez/sqlite`, pure Go, no CGO) instead of PostgreSQL;
- helpers `setupTestEnv`, `testMessage`, `testContext`, `testCallbackContext`, `seedUser`,
  `waitForCalls`.

Album saves to PM are tested with incoming messages and 128 concurrent save requests: each save
sorts its own snapshot, leaves the shared cache unchanged, and releases its lock before calling
Telegram. Empty-cache fallback and empty copy results are covered separately.

Most tests use local stand-ins and SQLite. `TestStatsPostgresConcurrent` additionally checks
concurrent dictionary creation and message, word and mute-time counters against real PostgreSQL:
32 concurrent workers, each updating every counter 20 times. CI provides a separate service
container; locally this test runs when `ZAVTRA_TEST_POSTGRES_DSN` points to a test database.
It creates a random isolated schema and drops it afterwards. Migrations, mute history and concurrent
game commands also run on PostgreSQL; other scenarios use SQLite.

`TestMessageStatsQueryCount` counts SQL statements in the message handler. With a warm dictionary,
10 distinct words require 1 statement instead of 15, and 10 repetitions of one word require
1 instead of 6, when the sender's data is also cached. BEGIN/COMMIT are excluded. All counters for one message commit in one atomic
batch without GORM's extra transaction; failure leaves no partially applied counters.
Keys are sorted for concurrent batches. Repetitions and inflected forms are combined
by existing dictionary ID, preserving first spellings, periods and historical rows. A FIFO cache
holds at most 4096 stems of up to 128 bytes, is scoped to its database, and coalesces concurrent
loads of one stem within the process. Errors and zero IDs are not cached. No database migration
or historical dictionary deduplication is needed.

Heavy media commands share a queue: 2 running jobs and up to 8 waiting for at most 30 seconds.
Each admitted job has a 5-minute processing deadline; a full queue fails immediately.
On Unix a timeout kills the process group, including yt-dlp's child FFmpeg processes.
`/distort` also retains its one-job limit. Ordinary commands do not wait on this queue.
Temporary outputs are removed on failure; conversions preserve shared Telegram source files.
The limit applies to jobs, not to FFmpeg's internal thread count.

User records are saved when any field changes. A database-pool-scoped FIFO cache holds at most
4096 users and serializes concurrent writes for the same user. It skips identical snapshots
for 5 minutes after a successful persistence; expiry revalidates with an upsert that only
physically updates changed PostgreSQL rows. Errors are retried and missing rows are repaired
after expiry. Empty strings and false flags are also saved. Messages and `chat_member` events
share this path.

Fourteen tests run the real `ffmpeg`/`ffprobe` over generated one-second
files and skip when the binaries are missing (`TestFFmpegConvert`,
`TestFFmpegConvertConcurrentSameName`, `TestConvert`, `TestInvert`, `TestDistort`,
`TestLoopAnimation`, `TestDownloadFile`, `TestFFmpegRemoteMedia`, `TestGetImageForLLM`,
`TestGetImageForLLMRemote`, `TestGetImageForLLMConcurrentSameFile`, `TestAIWithPhoto`,
`TestMp3LocalSource`, `TestDownloadLocalSource`); `TestMtprotoConnectProbe` skips without
`PROTO_PROBE=1` and `TestYtdlpGetVer` skips without an installed `yt-dlp`.

Temporary files of the conversions (`/convert`, `/mp3`, `/download`, the picture for `/ai`) and the
`/distort` work directories get unique names — `os.CreateTemp`, `chat_id` + `message_id`, and
`DistortTempDir` for tests. Several runs of the suite on one machine no longer wipe each other's
files, and concurrent requests for one and the same file no longer read someone else's copy;
`TestFFmpegConvertConcurrentSameName` and `TestGetImageForLLMConcurrentSameFile` prove it.

`/mp3` and `/download` run against the real `yt-dlp` on local media fixtures: `ytdlpFixtureServer`
builds a one-second m4a file and an HLS stream with separate video-only and audio-only variants,
serves them over HTTP on `127.0.0.1`, and `TestMain` takes loopback out of the proxy (`NO_PROXY`) so
`yt-dlp` reaches the fixture directly. `TestMp3LocalSource` and `TestDownloadLocalSource` check that
the quality format filter really finds the fixture, that `ffmpeg` merges/converts the tracks, and that
the file sent out is a finished mp3 and a finished mp4 carrying both tracks.

Tests never leave the machine: all outbound traffic goes to local stand-ins. `mitm_test.go` starts a
MITM proxy — `TestMain` generates a throwaway CA, points `SSL_CERT_FILE` at it and publishes the proxy
through `HTTP_PROXY`/`HTTPS_PROXY`, so even clients with the default transport (the CoinMarketCap
client) land in the stand-in; leaf certificates are signed per host on the fly. There are two
interception paths: `stubTLSProxy` (the proxy `httpClient()` uses) and `stubEnvRoute` (a route on the
process-wide proxy). The local `httptest` servers cover the Bot API (`TestBotInit` — `getMe`, webhook
deletion, polling and the sysadmin report; `TestBotInitWebhook` — the webhook server is up before
`setWebhook` is sent, and a request without the secret token gets 401) and the OpenAI format for `/ai`
(`TestAIStream`, `TestAIStreamError`, `TestAIWithPhoto`). Special tricks: `TestRestart` reruns the
process (`os.Exit` inside the handler cannot be checked from the same process), and `TestConfigInit`
works inside `t.TempDir()` with a cleared environment (`clearZavtraEnv`) so the `.env` of the
checkout is never touched. `TestStartupReport` checks the startup notice for secrets: the token is not
in it, and the passwords are stripped from the DSN, the proxy, the app hash and the API keys.

## Layout

```
main.go              handler configuration and startup
utils_bot.go         bot init, polling/webhook, middleware + anti-spam
utils_bot_proto.go   MTProto client: connect, reconnect, timeouts
utils_config.go      Configuration + the ZAVTRA_* environment variables (.env)
utils_db.go          gorm models, AutoMigrate, the "yes/no" text pools
utils_handlers.go    update handlers: user update, album cache, ban check, reactions
utils_user.go        user names and mentions, admin lists, finding a user in a message and in the DB
utils_stats.go       message, word and mute-time counters, start of day
utils_reply.go       Reply, ReplyAndRemove and SysAdminReply — how commands answer the user and the sysadmin
utils_startup.go     the startup notice: build, host, connection, database, chats, tool versions (no credentials)
utils_common.go      RandInt, StringInSlice, MarshalOmitEmptyAll
utils_http.go        HTTP client with proxy and timeout, DownloadFile
utils_forward.go     forwarding channel posts
utils_files.go       go:embed of stickers and the /megamlem image (assets/*.png, assets/megamlem.jpg)
utils_plural.go      Russian numerals (ruPlural)
media.go             the media of a message: its type, its file, where it is on disk
media_ffmpeg.go      FFmpegConvert — the conversion for /convert, /distort, /invert, /loop
command_*.go         commands
duel_*.go            duel: request/accept/stats
pidor_*.go           the "Pidor of the Day!" game
bets_*.go            bets
ai_*.go              AI module (disabled)
*_test.go            tests + harness
assets/              stickers, the /megamlem image (go:embed), and CREDITS.md
```

## MTProto client

`/meow`, `/woof` and `/meme` work through MTProto (`gotgproto` + `gotd`) rather than the Bot API:

- the client starts in a background goroutine, the connection attempt is bounded by
  `protoConnectTimeout` (45 s), on failure it retries every `protoReconnectDelay` (30 s); the log shows
  `MTProto client connected` or the reason (`MTProto client is not connected (attempt=N, error=…)`);
- with the client not connected, the commands answer "the MTProto client has not connected yet"
  instead of failing;
- every request has its own deadline (60 s for RPC, 3 min for downloads), read-only requests are retried;
- `NoUpdates: true` — the bot does not take updates away from the Bot API long polling, MTProto is used
  for RPC only;
- `ZAVTRA_PROXY` does **not** affect MTProto: datacenters are spoken to over direct TCP, so these
  commands need direct access to `*:443` (or a self-hosted MTProto transit).

## Known quirks

- `ZAVTRA_PROXY` applies only to the bot's ordinary HTTP requests (advice/anekdot/releases/file downloads);
  Bot API and MTProto calls do not use it.
- `/restart` calls `os.Exit(0)` — the restart must be handled by an external supervisor (`Restart=always`
  in systemd, `restart: always` in compose).
- The dispatcher sends handler errors to the chat ("Ошибка: …" plus the context: the command
  text, the user and the chat), a token in the text is replaced with
  `TOKEN`.
- The AI commands (`/ai`, `/restartai`, `/setaisystem`) are commented out in `main.go`.

## Development

Since **16 October 2026** development is done by AI: a model writes the changes, a human sets the
tasks, makes the decisions and reviews the result. Every such commit is marked in its message
(`Made with the … model.`) and as a co-author (`Co-authored-by: … <noreply@opencode.ai>`), so the AI
contribution is visible directly in the git history.

Models of the local Strata stack — configurations in `~/.strata/*.json`, engine
`~/.strata/engine/strata`, weights in `~/Strata-data/models`:

| Model (`model_name`) | What it is | In this repository |
|---|---|---|
| `swift-1.5-iq3_xxs` | Swift-1.5 Qwen3.8-Flash-Next GSQ RCO, IQ3_XXS quant (`ukisai/Swift-1.5-Qwen3.8-Flash-Next-GSQ-RCO-GGUF`) | current one: test harness, MTProto/plural fixes, command tests, real `yt-dlp` tests for `/mp3` and `/download`, `.env` configuration, README |
| `qwen3.8-flash-next-iq3_xxs` | Qwen3.8-Flash-Next GSQ RCO, IQ3_XXS quant (`ISTA-DASLab/Qwen3.8-Flash-Next-GSQ-RCO-GGUF`) | `e55e505` (go:embed), `30415bd` (helper tests + `RandInt` fix) |
| `qwen3.8-flash-next` | the same model, before the separate quant | `4ab27c5` — the first AI commit (`bot_api_url` for all API calls) |
| `orcarouter-qwen3.8-flash-next-uncensored-iq3_xxs` | Qwen3.8-Flash-Next Uncensored, IQ3_XXS | configured in `~/.strata`, not applied to this code |

Alongside them the stack also has the vision projections `mmproj-Qwen3.8-Flash-Next-BF16.gguf` and
`mmproj-Swift-Qwen3.8-Flash-Next-BF16.gguf`, a draft model for speculative decoding `mtp-q2_0.gguf`,
and `~/.strata/data/gguf_fingerprints.json` describes other quantizations of the same models (Q2_0,
IQ2_XS, IQ3_XXS, IQ3_S, coder IQ1_M, unsloth UD-IQ4_XS / UD-Q4_K_XL).

## License

MIT (see `LICENSE`), © 2026 NexonSU. The license covers the source code of the bot.

What it does not cover:

- `assets/bonk.png` and `assets/hug.png` — third-party memes, their rights are not granted; details in
  [`assets/CREDITS.en.md`](assets/CREDITS.en.md). Before publishing the project, remove them or replace
  them with your own.
- `ffmpeg` and `yt-dlp` — external GPL programs the bot runs as separate processes; the bot's license
  does not extend to them, their own license terms apply.
- Almost all Go dependencies are MIT / BSD-3 / Apache-2.0 and compatible with MIT. The exception is
  `github.com/celestix/gotgproto` under **GPL-3.0**: it is linked into the binary, so distributing the
  built bot puts the whole work under GPL-3 (the sources have to be disclosed). Running the bot on your
  own server carries no obligation: GPL triggers on distribution, not on use.

Salvation phrase commands use the configured admin/moderator roles (`ZAVTRA_ADMINS`, `ZAVTRA_MODERS`). This separate pool does not change blessing probability or statistics. The last phrase cannot be deleted.

`/blessing`, `/suicide` and `/isekai` have a 33% salvation chance: the user name and a random salvation phrase, followed by “Живи. Пока что.”. Salvation adds no death, mute time or blessing burst event. Admin ricochets always bless; an empty salvation pool falls back to blessing. Salvation counts appear in `/duelstats` and `/stats saves [day|week]`.
