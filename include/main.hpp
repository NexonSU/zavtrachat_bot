#include <cstdio>
#include <cstdlib>
#include <exception>
#include <string>
#include <tgbot/tgbot.h>
#include "utils.hpp"
#include "commands.hpp"

using namespace std;
using namespace TgBot;

utils::configuration config = utils::loadConfig("config.json");

Bot bot(config.token);