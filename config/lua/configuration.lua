local cjson = require("cjson.safe")
local configuration_data = ngx.shared.tcp_udp_configuration_data
local tostring = tostring

local _M = {
    TRUSTED_DOMAIN_LIST = { "snowinning.com" }
}

function _M.set_users(users)
    local ok, err = configuration_data:set("users", users)
    if not ok then
        return "failed to updating users configuration, " .. tostring(err)
    end
    return ""
end

local function get_users()
    local users_data, get_user_err = configuration_data:get("users")
    if not users_data then
        ngx.log(ngx.ERR, "get_users(): could not to get users data, " .. tostring(get_user_err))
        return
    end

    local users = cjson.decode(users_data)
    if not users then
        ngx.log(ngx.ERR, "get_users(): cjson.decode err, could not parse users data")
        return
    end

    return users
end

function _M.list_users()
    return get_users()
end

function _M.get_user(name)
    if name == nil or name == "" then
        return
    end

    local users = get_users()
    if not users then
        return
    end

    for _, user in ipairs(users) do
        if name == user.name then
            return user
        end
    end
    return
end

return _M
