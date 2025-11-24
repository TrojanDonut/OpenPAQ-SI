-- Simple delay function for HAProxy
core.register_action("delay", { "http-req" }, function(txn)
    -- Add a 2 second delay to give ClickHouse time to respond first
    -- ClickHouse queries should be near-instant, so this ensures they win the race
    core.msleep(2000)
end)

