package redis

const depositLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  return {"DUPLICATE", redis.call("GET", KEYS[1]), "duplicate request ignored"}
end
if redis.call("EXISTS", KEYS[2]) == 0 then
  return {"NOT_FOUND", "", "wallet not found"}
end
redis.call("HINCRBY", KEYS[2], "balance_cents", ARGV[1])
redis.call("HSET", KEYS[3],
  "id", ARGV[2],
  "type", "DEPOSIT",
  "to_wallet_id", string.sub(KEYS[2], 8),
  "amount_cents", ARGV[1],
  "status", "SUCCESS",
  "message", "deposit success",
  "created_at", ARGV[3]
)
redis.call("SET", KEYS[1], ARGV[2], "EX", 86400)
return {"OK", ARGV[2], "deposit success"}
`

const withdrawLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  return {"DUPLICATE", redis.call("GET", KEYS[1]), "duplicate request ignored"}
end
if redis.call("EXISTS", KEYS[2]) == 0 then
  return {"NOT_FOUND", "", "wallet not found"}
end
local balance = tonumber(redis.call("HGET", KEYS[2], "balance_cents") or "0")
local amount = tonumber(ARGV[1])
if balance < amount then
  return {"INSUFFICIENT_FUNDS", "", "insufficient funds"}
end
redis.call("HINCRBY", KEYS[2], "balance_cents", -amount)
redis.call("HSET", KEYS[3],
  "id", ARGV[2],
  "type", "WITHDRAW",
  "from_wallet_id", string.sub(KEYS[2], 8),
  "amount_cents", ARGV[1],
  "status", "SUCCESS",
  "message", "withdraw success",
  "created_at", ARGV[3]
)
redis.call("SET", KEYS[1], ARGV[2], "EX", 86400)
return {"OK", ARGV[2], "withdraw success"}
`

const transferLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  return {"DUPLICATE", redis.call("GET", KEYS[1]), "duplicate request ignored"}
end
if redis.call("EXISTS", KEYS[2]) == 0 then
  return {"FROM_NOT_FOUND", "", "from wallet not found"}
end
if redis.call("EXISTS", KEYS[3]) == 0 then
  return {"TO_NOT_FOUND", "", "to wallet not found"}
end
local balance = tonumber(redis.call("HGET", KEYS[2], "balance_cents") or "0")
local amount = tonumber(ARGV[1])
if balance < amount then
  return {"INSUFFICIENT_FUNDS", "", "insufficient funds"}
end
redis.call("HINCRBY", KEYS[2], "balance_cents", -amount)
redis.call("HINCRBY", KEYS[3], "balance_cents", amount)
redis.call("HSET", KEYS[4],
  "id", ARGV[2],
  "type", "TRANSFER",
  "from_wallet_id", string.sub(KEYS[2], 8),
  "to_wallet_id", string.sub(KEYS[3], 8),
  "amount_cents", ARGV[1],
  "status", "SUCCESS",
  "message", "transfer success",
  "created_at", ARGV[3]
)
redis.call("SET", KEYS[1], ARGV[2], "EX", 86400)
return {"OK", ARGV[2], "transfer success"}
`
