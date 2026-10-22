package redis

const createWalletLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  return "ALREADY_EXISTS"
end
redis.call("HSET", KEYS[1],
  "id", ARGV[1],
  "user_id", ARGV[2],
  "balance_cents", ARGV[3],
  "created_at", ARGV[4]
)
redis.call("SADD", KEYS[2], ARGV[1])
if tonumber(ARGV[3]) > 0 then
  redis.call("HSET", KEYS[3],
    "id", ARGV[5],
    "type", "DEPOSIT",
    "to_wallet_id", ARGV[1],
    "amount_cents", ARGV[3],
    "status", "SUCCESS",
    "message", "initial balance",
    "created_at", ARGV[4]
  )
end
return "OK"
`

const depositLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  if redis.call("HGET", KEYS[1], "fingerprint") ~= ARGV[4] then
    return {"IDEMPOTENCY_CONFLICT", "", "request id payload mismatch", ARGV[3]}
  end
  return {"DUPLICATE", redis.call("HGET", KEYS[1], "transaction_id"), "duplicate request ignored", redis.call("HGET", KEYS[1], "created_at")}
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
redis.call("HSET", KEYS[1], "transaction_id", ARGV[2], "fingerprint", ARGV[4], "created_at", ARGV[3])
redis.call("EXPIRE", KEYS[1], 86400)
return {"OK", ARGV[2], "deposit success", ARGV[3]}
`

const withdrawLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  if redis.call("HGET", KEYS[1], "fingerprint") ~= ARGV[4] then
    return {"IDEMPOTENCY_CONFLICT", "", "request id payload mismatch", ARGV[3]}
  end
  return {"DUPLICATE", redis.call("HGET", KEYS[1], "transaction_id"), "duplicate request ignored", redis.call("HGET", KEYS[1], "created_at")}
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
redis.call("HSET", KEYS[1], "transaction_id", ARGV[2], "fingerprint", ARGV[4], "created_at", ARGV[3])
redis.call("EXPIRE", KEYS[1], 86400)
return {"OK", ARGV[2], "withdraw success", ARGV[3]}
`

const transferLua = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  if redis.call("HGET", KEYS[1], "fingerprint") ~= ARGV[4] then
    return {"IDEMPOTENCY_CONFLICT", "", "request id payload mismatch", ARGV[3]}
  end
  return {"DUPLICATE", redis.call("HGET", KEYS[1], "transaction_id"), "duplicate request ignored", redis.call("HGET", KEYS[1], "created_at")}
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
redis.call("HSET", KEYS[1], "transaction_id", ARGV[2], "fingerprint", ARGV[4], "created_at", ARGV[3])
redis.call("EXPIRE", KEYS[1], 86400)
return {"OK", ARGV[2], "transfer success", ARGV[3]}
`
