#include "utils/config.hpp"

utils::configuration utils::loadConfig(const std::string &filename)
{
    utils::configuration config;

    if (!std::filesystem::exists(filename))
    {
        pt::ptree new_root;
        new_root.add("token", "REQUIRED! String: Telegram bot token.");
        new_root.add("app_id", 0);
        new_root.add("app_hash", "");
        new_root.add("chat", 0);
        new_root.add("reserve_chat", 0);
        new_root.add("comment_chat", 0);
        new_root.add("channel", 0);
        new_root.add("stream_channel", 0);
        new_root.add("bot_api_url", "https://api.telegram.org");
        pt::ptree admins, admin1, admin2, admin3;
        admin1.put("", 1);
        admin2.put("", 2);
        admin3.put("", 3);
        admins.push_back(std::make_pair("", admin1));
        admins.push_back(std::make_pair("", admin2));
        admins.push_back(std::make_pair("", admin3));
        new_root.add_child("admins", admins);
        new_root.add_child("moders", admins);
        new_root.add("sysadmin", 123);
        pt::ptree allowed_updates, au1, au2, au3, au4, au5, au6;
        au1.put("", "message");
        au2.put("", "channel_post");
        au3.put("", "edited_channel_post");
        au4.put("", "callback_query");
        au5.put("", "chat_member");
        au6.put("", "inline_query");
        allowed_updates.push_back(std::make_pair("", au1));
        allowed_updates.push_back(std::make_pair("", au2));
        allowed_updates.push_back(std::make_pair("", au3));
        allowed_updates.push_back(std::make_pair("", au4));
        allowed_updates.push_back(std::make_pair("", au5));
        allowed_updates.push_back(std::make_pair("", au6));
        new_root.add_child("allowed_updates", allowed_updates);
        new_root.add("listen", "");
        new_root.add("endpoint_public_url", "");
        new_root.add("max_connections", 10000);
        new_root.add("currency_key", "");
        new_root.add("releases_url", "");
        pt::write_json(filename, new_root);
    }

    pt::ptree root;
    pt::read_json(filename, root);

    config.token = root.get<std::string>("token");
    config.app_id = root.get<int>("app_id");
    config.app_hash = root.get<std::string>("app_hash");
    config.chat = root.get<int64_t>("chat");
    config.reserve_chat = root.get<int64_t>("reserve_chat");
    config.comment_chat = root.get<int64_t>("comment_chat");
    config.channel = root.get<int64_t>("channel");
    config.stream_channel = root.get<int64_t>("stream_channel");
    config.bot_api_url = root.get<std::string>("bot_api_url");
    for (auto& admin : root.get_child("admins"))
        config.admins.push_back(admin.second.get_value<int64_t>());
    for (auto& moder : root.get_child("moders"))
        config.moders.push_back(moder.second.get_value<int64_t>());
    config.sysadmin = root.get<int64_t>("sysadmin");
    for (auto& moder : root.get_child("allowed_updates"))
        config.allowed_updates.push_back(moder.second.get_value<std::string>());
    config.listen = root.get<std::string>("listen");
    config.endpoint_public_url = root.get<std::string>("endpoint_public_url");
    config.max_connections = root.get<int>("max_connections");
    config.currency_key = root.get<std::string>("currency_key");
    config.releases_url = root.get<std::string>("releases_url");

    if (config.token == "REQUIRED! String: Telegram bot token.") {
        printf("Change bot token in config.json.\n");
        exit(0);
    }

    return config;
}