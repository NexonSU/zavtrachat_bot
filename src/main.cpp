#include "main.hpp"

int main() {
    utils::config.load("config.json");

    Bot bot(utils::config.token);
    bot.getEvents().onCommand("marco", [&bot](Message::Ptr message) {
        commands::marco(&bot, message);
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
