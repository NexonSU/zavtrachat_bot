package main

// The bot is configured by the environment. Real environment variables win over
// the .env file, and the .env file is the way Docker and systemd configure it:
// `docker run --env-file .env`, compose `env_file: .env`, systemd `EnvironmentFile=`.

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// DefaultReleasesUrl is the release calendar used by /releases when
// ZAVTRA_RELEASES_URL is not set.
const DefaultReleasesUrl = "https://ical-videogames.onrender.com/calendar?platform=ps4&platform=ps5&platform=switch&platform=xbox_one&platform=xbox_series&region=pal"

// DefaultBotApiUrl is the Bot API endpoint used when ZAVTRA_BOT_API_URL is not
// set: the hosted api.telegram.org.
const DefaultBotApiUrl = "https://api.telegram.org"

// DefaultAllowedUpdates is what the bot asks Telegram to deliver when
// ZAVTRA_ALLOWED_UPDATES is not set.
var DefaultAllowedUpdates = []string{"message", "channel_post", "callback_query", "chat_member"}

// DefaultEnvFile is the file ConfigInit loads from the working directory when
// ZAVTRA_ENV_FILE does not point somewhere else.
const DefaultEnvFile = ".env"

// Configuration is the whole bot configuration. Every field is a ZAVTRA_*
// environment variable of the same name in upper snake case: Token is
// ZAVTRA_TOKEN, BotApiDataDir is ZAVTRA_BOT_API_DATADIR, and so on.
type Configuration struct {
	Token                   string
	DSN                     string
	AppID                   int
	AppHash                 string
	BotApiUrl               string
	BotApiDataDir           string
	AllowedUpdates          []string
	Listen                  string
	EndpointPublicURL       string
	MaxConnections          int64
	Chat                    int64
	ReserveChat             int64
	CommentChat             int64
	StreamChannel           int64
	Channel                 int64
	Admins                  []int64
	Moders                  []int64
	SysAdmin                int64
	CurrencyKey             string
	ReleasesUrl             string
	FontPath                string
	Proxy                   string
	AIURL                   string
	AIToken                 string
	AIModel                 string
	AISystem                string
	AIAdminOnlyTools        []string
	AntiSpamWindow          int
	AntiSpamCount           int
	QuoteProtectedUsernames []string
}

// ConfigInit builds the configuration from the environment: it loads the .env
// file first (ZAVTRA_ENV_FILE points somewhere else, a missing file is fine),
// then reads the variables. A value that is already in the environment is not
// overwritten by the file, so the process environment always wins.
func ConfigInit() error {
	envFile := os.Getenv("ZAVTRA_ENV_FILE")
	if envFile == "" {
		envFile = DefaultEnvFile
	}
	if err := loadEnvFile(envFile); err != nil {
		return err
	}

	cfg := &Configuration{
		BotApiUrl:      envValue("ZAVTRA_BOT_API_URL", DefaultBotApiUrl),
		BotApiDataDir:  os.Getenv("ZAVTRA_BOT_API_DATADIR"),
		AllowedUpdates: envList("ZAVTRA_ALLOWED_UPDATES", DefaultAllowedUpdates),
		Admins:         []int64{},
		Moders:         []int64{},
		ReleasesUrl:    envValue("ZAVTRA_RELEASES_URL", DefaultReleasesUrl),
	}

	cfg.QuoteProtectedUsernames = envList("ZAVTRA_QUOTE_PROTECTED_USERNAMES", strings.Split(quoteProtectedDefaults, ","))
	for i := len(cfg.QuoteProtectedUsernames) - 1; i >= 0; i-- {
		if strings.TrimSpace(cfg.QuoteProtectedUsernames[i]) == "" {
			cfg.QuoteProtectedUsernames = append(cfg.QuoteProtectedUsernames[:i], cfg.QuoteProtectedUsernames[i+1:]...)
		}
	}
	cfg.Token = os.Getenv("ZAVTRA_TOKEN")
	cfg.DSN = os.Getenv("ZAVTRA_DSN")
	cfg.AppHash = os.Getenv("ZAVTRA_APP_HASH")
	cfg.Listen = os.Getenv("ZAVTRA_LISTEN")
	cfg.EndpointPublicURL = os.Getenv("ZAVTRA_ENDPOINT_PUBLIC_URL")
	cfg.CurrencyKey = os.Getenv("ZAVTRA_CURRENCY_KEY")
	cfg.FontPath = os.Getenv("ZAVTRA_FONTPATH")
	cfg.Proxy = os.Getenv("ZAVTRA_PROXY")
	cfg.AIURL = os.Getenv("ZAVTRA_AI_URL")
	cfg.AIToken = os.Getenv("ZAVTRA_AI_TOKEN")
	cfg.AIModel = os.Getenv("ZAVTRA_AI_MODEL")
	cfg.AISystem = os.Getenv("ZAVTRA_AI_SYSTEM")
	cfg.AIAdminOnlyTools = envList("ZAVTRA_AI_ADMIN_ONLY_TOOLS", nil)

	appID, err := envInt("ZAVTRA_APP_ID")
	if err != nil {
		return err
	}
	cfg.AppID = appID
	antiSpamWindow, err := envInt("ZAVTRA_ANTISPAM_WINDOW")
	if err != nil {
		return err
	}
	cfg.AntiSpamWindow = antiSpamWindow
	antiSpamCount, err := envInt("ZAVTRA_ANTISPAM_COUNT")
	if err != nil {
		return err
	}
	cfg.AntiSpamCount = antiSpamCount

	maxConnections, err := envInt64("ZAVTRA_MAX_CONNECTIONS")
	if err != nil {
		return err
	}
	cfg.MaxConnections = maxConnections
	chat, err := envInt64("ZAVTRA_CHAT")
	if err != nil {
		return err
	}
	cfg.Chat = chat
	reserveChat, err := envInt64("ZAVTRA_RESERVE_CHAT")
	if err != nil {
		return err
	}
	cfg.ReserveChat = reserveChat
	commentChat, err := envInt64("ZAVTRA_COMMENT_CHAT")
	if err != nil {
		return err
	}
	cfg.CommentChat = commentChat
	streamChannel, err := envInt64("ZAVTRA_STREAM_CHANNEL")
	if err != nil {
		return err
	}
	cfg.StreamChannel = streamChannel
	channel, err := envInt64("ZAVTRA_CHANNEL")
	if err != nil {
		return err
	}
	cfg.Channel = channel
	sysAdmin, err := envInt64("ZAVTRA_SYSADMIN")
	if err != nil {
		return err
	}
	cfg.SysAdmin = sysAdmin
	admins, err := envInt64List("ZAVTRA_ADMINS", cfg.Admins)
	if err != nil {
		return err
	}
	cfg.Admins = admins
	moders, err := envInt64List("ZAVTRA_MODERS", cfg.Moders)
	if err != nil {
		return err
	}
	cfg.Moders = moders

	if cfg.Token == "" {
		return fmt.Errorf("ZAVTRA_TOKEN is not set: put the bot token in .env or in the environment")
	}
	if cfg.Chat == 0 {
		return fmt.Errorf("ZAVTRA_CHAT is not set: put the main chat id in .env or in the environment")
	}
	if cfg.Proxy != "" {
		proxyUrl, err := url.Parse(cfg.Proxy)
		if err != nil {
			return err
		}
		HTTPClientProxy = http.ProxyURL(proxyUrl)
	}

	Config = cfg
	return nil
}

// loadEnvFile loads a .env file into the environment. A file that is not there
// is not an error: the environment alone is a complete configuration. Values
// already in the environment are kept, so the file only fills the gaps.
func loadEnvFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

// envValue returns the variable, or def when it is not set.
func envValue(name string, def string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return def
}

// envInt and envInt64 read a number. An unset variable is 0; anything that is
// not a number is an error naming the variable, because "ZAVTRA_CHAT=x" in a
// .env file gives no hint about where it came from otherwise.
func envInt(name string) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer", name, value)
	}
	return number, nil
}

func envInt64(name string) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, nil
	}
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer", name, value)
	}
	return number, nil
}

// envList reads a comma separated list; spaces and semicolons separate too,
// because "ZAVTRA_ADMINS=1, 2, 3" is what people write in a .env file.
func envList(name string, def []string) []string {
	value := os.Getenv(name)
	if value == "" {
		return def
	}
	return splitList(value)
}

func envInt64List(name string, def []int64) ([]int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return def, nil
	}
	numbers := []int64{}
	for _, part := range splitList(value) {
		number, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not an integer", name, part)
		}
		numbers = append(numbers, number)
	}
	return numbers, nil
}

func splitList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t'
	})
}
