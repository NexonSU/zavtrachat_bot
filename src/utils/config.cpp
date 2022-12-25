#include "utils/config.hpp"

void utils::configuration::load(const std::string &filename)
{
    pt::ptree root;
    pt::read_json(filename, root);
    token = root.get<std::string>("token");
    app_id = root.get<int>("app_id");
    app_hash = root.get<std::string>("app_hash");
    chat = root.get<int64_t>("chat");
    reserve_chat = root.get<int64_t>("reserve_chat");
    comment_chat = root.get<int64_t>("comment_chat");
    channel = root.get<int64_t>("channel");
    stream_channel = root.get<int64_t>("stream_channel");
    bot_api_url = root.get<std::string>("bot_api_url");
    for (auto& admin : root.get_child("admins"))
        admins.push_back(admin.second.get_value<int64_t>());
    for (auto& moder : root.get_child("moders"))
        moders.push_back(moder.second.get_value<int64_t>());
    sysadmin = root.get<int64_t>("sysadmin");
    for (auto& moder : root.get_child("allowed_updates"))
        allowed_updates.push_back(moder.second.get_value<std::string>());
    listen = root.get<std::string>("listen");
    endpoint_public_url = root.get<std::string>("endpoint_public_url");
    max_connections = root.get<int>("max_connections");
    currency_key = root.get<std::string>("currency_key");
    releases_url = root.get<std::string>("releases_url");
}