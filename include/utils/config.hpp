#include <boost/property_tree/ptree.hpp>
#include <boost/property_tree/json_parser.hpp>
#include <string>
#include <set>
#include <exception>
#include <iostream>
namespace pt = boost::property_tree;

namespace utils 
{
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
    };

    configuration loadConfig(const std::string &filename);
}