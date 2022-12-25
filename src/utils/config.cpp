#include "utils/config.hpp"

utils::configuration utils::loadConfig(const std::string &filename)
{
    pt::ptree root;
    pt::read_json(filename, root);

    utils::configuration config;

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

    return config;
}