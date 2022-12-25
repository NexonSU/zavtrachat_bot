#include "commands/marco.hpp"

void commands::marco(TgBot::Message::Ptr message) 
{
    bot.getApi().sendMessage(message->chat->id, "Polo!", true, message->messageId);
}