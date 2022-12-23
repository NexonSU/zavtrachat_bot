#include <cstdio>
#include <cstdlib>
#include <exception>
#include <string>
#include <tgbot/tgbot.h>
#include "utils.h"
#include "commands.h"

using namespace std;
using namespace TgBot;

int main() {
    configuration config;
    config.load("config.json");
    string token(config.token);

    Bot bot(token);
    bot.getEvents().onCommand("marco", [&bot](Message::Ptr message) {
        bot.getApi().sendMessage(message->chat->id, Marco(), true, message->messageId);
    });

    signal(SIGINT, [](int s) {
        printf("SIGINT got\n");
        exit(0);
    });

    try {
        printf("Bot username: %s\n", bot.getApi().getMe()->username.c_str());
        bot.getApi().deleteWebhook();

        TgLongPoll longPoll(bot);
        while (true) {
            printf("Long poll started\n");
            longPoll.start();
        }
    } catch (exception& e) {
        printf("error: %s\n", e.what());
    }

    return 0;
}
