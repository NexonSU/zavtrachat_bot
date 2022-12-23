#include <boost/property_tree/ptree.hpp>
#include <boost/property_tree/json_parser.hpp>
#include <string>
#include <set>
#include <exception>
#include <iostream>
namespace pt = boost::property_tree;

struct configuration
{
    std::string token;
    int app_id;
    std::string app_hash;
    int64_t chat;
    int64_t reserve_chat;
    int64_t comment_chat;
    int64_t channel;
    int64_t stream_channel;
    std::string bot_api_url;
    std::vector<int64_t> admins;
    std::vector<int64_t> moders;
    int64_t sysadmin;
    std::vector<std::string> allowed_updates;
    std::string listen;
    std::string endpoint_public_url;
    int max_connections;
    std::string currency_key;
    std::string releases_url;
    void load(const std::string &filename);
};

void configuration::load(const std::string &filename)
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