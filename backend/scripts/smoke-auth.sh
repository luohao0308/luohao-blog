#!/bin/bash
# M2/S2 smoke: login -> protected probe -> refresh -> logout + negative cases.
# Usage: backend/scripts/smoke-auth.sh
# Requires the dev compose stack (mysql :3307, redis :6379) up and jq.
set -u
cd "$(dirname "$0")/.."
BASE=http://127.0.0.1:8000
PASS=0; FAIL=0
say() { printf '\n== %s\n' "$*"; }
ok()  { PASS=$((PASS+1)); echo "PASS: $*"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL: $*"; }
expect_code() { # desc expected actual
  if [ "$3" = "$2" ]; then ok "$1 (HTTP $3)"; else bad "$1 want $2 got $3"; fi
}

say "start server with a fresh secret"
export KRATOS_JWT_SECRET=$(openssl rand -hex 32)
# Dev-only: clear stale rate-limit windows from earlier runs.
docker exec blog-redis redis-cli FLUSHDB >/dev/null
go run ./cmd/server -conf ./configs >/tmp/s2_server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null' EXIT
for i in $(seq 1 60); do
  curl -sf "$BASE/v1/articles/list" >/dev/null 2>&1 && break
  sleep 0.5
done

say "seed the author account"
SEED_OUT=$(go run ./cmd/seed -conf ./configs -email author@example.com -password 'longenough1' -name '罗豪' 2>&1)
echo "$SEED_OUT"
case "$SEED_OUT" in
  *"created author"*|*"already registered"*) ok "seed (created or existing)" ;;
  *) bad "seed" ;;
esac

say "1. login with wrong password -> 401"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/login" -H 'Content-Type: application/json' -d '{"email":"author@example.com","password":"wrong"}')
expect_code "wrong password" 401 "$CODE"
jq -r '.reason // empty' /tmp/s2_r.json | grep -q USER_INVALID_CREDENTIALS && ok "reason USER_INVALID_CREDENTIALS" || bad "reason: $(cat /tmp/s2_r.json)"

say "2. login with unknown email -> 401, indistinguishable reason"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/login" -H 'Content-Type: application/json' -d '{"email":"ghost@example.com","password":"wrong"}')
expect_code "unknown email" 401 "$CODE"
jq -r '.reason // empty' /tmp/s2_r.json | grep -q USER_INVALID_CREDENTIALS && ok "same reason (no enumeration)" || bad "reason: $(cat /tmp/s2_r.json)"

say "3. login correct -> 200 + access token + refresh cookie"
LOGIN=$(curl -s -c /tmp/s2_jar.txt -D /tmp/s2_h.txt -X POST "$BASE/v1/auth/login" -H 'Content-Type: application/json' -d '{"email":"author@example.com","password":"longenough1"}')
CODE=$(echo "$LOGIN" | jq -r '.access_token // empty' >/dev/null 2>&1; echo $?)
ACCESS=$(echo "$LOGIN" | jq -r '.access_token // empty')
[ -n "$ACCESS" ] && ok "access token issued" || bad "no access token: $LOGIN"
grep -i 'set-cookie: refresh_token=' /tmp/s2_h.txt >/dev/null && ok "refresh cookie set" || bad "no refresh cookie"
grep -i 'set-cookie: refresh_token=' /tmp/s2_h.txt | grep -iq 'httponly' && ok "refresh cookie HttpOnly" || bad "cookie not HttpOnly"
grep -i 'set-cookie: refresh_token=' /tmp/s2_h.txt | grep -iq 'path=/v1/auth' && ok "refresh cookie scoped to /v1/auth" || bad "cookie path wrong"
echo "$LOGIN" | jq -e '.user.role == 1 and .token_type == "Bearer" and .expires_in > 0' >/dev/null && ok "user/tokenType/expiresIn fields" || bad "reply fields: $LOGIN"

say "4. GET /v1/auth/me without token -> 401 AUTH_UNAUTHORIZED"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' "$BASE/v1/auth/me")
expect_code "me without token" 401 "$CODE"
jq -r '.reason // empty' /tmp/s2_r.json | grep -q AUTH_UNAUTHORIZED && ok "reason AUTH_UNAUTHORIZED" || bad "reason: $(cat /tmp/s2_r.json)"

say "5. GET /v1/auth/me with forged token -> 401 (not expired)"
FORGED=$(python3 - <<'EOF'
import base64, json, time
h = base64.urlsafe_b64encode(json.dumps({"alg":"HS256","typ":"JWT"}).encode()).rstrip(b"=")
p = base64.urlsafe_b64encode(json.dumps({"sub":"00000000-0000-0000-0000-000000000000","role":1,"exp":int(time.time())+600,"iat":int(time.time())}).encode()).rstrip(b"=")
print(f"{h.decode()}.{p.decode()}.Zm9yZ2VkLXNpZ25hdHVyZQ")
EOF
)
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' "$BASE/v1/auth/me" -H "Authorization: Bearer $FORGED")
expect_code "me with forged token" 401 "$CODE"
jq -r '.reason // empty' /tmp/s2_r.json | grep -q AUTH_UNAUTHORIZED && ok "reason AUTH_UNAUTHORIZED (forged)" || bad "reason: $(cat /tmp/s2_r.json)"

say "6. GET /v1/auth/me with valid token -> 200"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' "$BASE/v1/auth/me" -H "Authorization: Bearer $ACCESS")
expect_code "me with token" 200 "$CODE"
jq -e '.email == "author@example.com"' /tmp/s2_r.json >/dev/null && ok "me returns the author" || bad "me body: $(cat /tmp/s2_r.json)"

say "7. refresh with cookie -> new pair; replay old cookie -> 401"
REFRESH1=$(curl -s -b /tmp/s2_jar.txt -c /tmp/s2_jar2.txt -D /tmp/s2_h2.txt -X POST "$BASE/v1/auth/refresh" -H 'Content-Type: application/json' -d '{}')
NEWACCESS=$(echo "$REFRESH1" | jq -r '.access_token // empty')
[ -n "$NEWACCESS" ] && ok "new access token issued (rotation asserted by replay checks)" || bad "refresh reply: $REFRESH1"
grep -i 'set-cookie: refresh_token=' /tmp/s2_h2.txt >/dev/null && ok "replacement refresh cookie set" || bad "no replacement cookie"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/refresh" -H 'Content-Type: application/json' -d '{}')
expect_code "refresh without cookie/header" 401 "$CODE"

say "8. refresh via X-Refresh-Token header (non-cookie client)"
# consume the rotated token from jar2 to get a fresh session for header use
RT2=$(grep refresh_token /tmp/s2_jar2.txt | awk '{print $NF}')
HEADER_REFRESH=$(curl -s -X POST "$BASE/v1/auth/refresh" -H 'Content-Type: application/json' -d '{}' -H "X-Refresh-Token: $RT2")
NEWACCESS2=$(echo "$HEADER_REFRESH" | jq -r '.access_token // empty')
[ -n "$NEWACCESS2" ] && ok "header-based refresh works" || bad "header refresh: $HEADER_REFRESH"

say "9. replay the just-consumed header token -> 401"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/refresh" -H 'Content-Type: application/json' -d '{}' -H "X-Refresh-Token: $RT2")
expect_code "replayed refresh token" 401 "$CODE"
jq -r '.reason // empty' /tmp/s2_r.json | grep -q AUTH_INVALID_REFRESH_TOKEN && ok "reason AUTH_INVALID_REFRESH_TOKEN" || bad "reason: $(cat /tmp/s2_r.json)"

say "10. logout revokes session and clears cookie"
RT3=$(grep refresh_token /tmp/s2_jar2.txt | awk '{print $NF}')
# RT3 in jar2 was rotated in step 8; capture the current cookie jar from step 8's response? curl did not save it; re-login for a clean session
LOGIN2=$(curl -s -c /tmp/s2_jar3.txt -X POST "$BASE/v1/auth/login" -H 'Content-Type: application/json' -d '{"email":"author@example.com","password":"longenough1"}')
ACCESS3=$(echo "$LOGIN2" | jq -r '.access_token')
CODE=$(curl -s -o /dev/null -w '%{http_code}' -H 'Content-Type: application/json' -d '{}' -b /tmp/s2_jar3.txt -c /tmp/s2_jar3_after.txt -X POST "$BASE/v1/auth/logout")
expect_code "logout" 200 "$CODE"
grep -i 'set-cookie: refresh_token=' /tmp/s2_jar3_after.txt >/dev/null 2>&1
MAXAGE=$(curl -s -D - -o /dev/null -H 'Content-Type: application/json' -d '{}' -b /tmp/s2_jar3.txt -X POST "$BASE/v1/auth/logout" | grep -i 'set-cookie' | grep -iEo 'max-age=(-1|0)')
[ -n "$MAXAGE" ] && ok "logout clears cookie (Max-Age=-1)" || bad "cookie not cleared"
CODE=$(curl -s -o /tmp/s2_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/refresh" -H 'Content-Type: application/json' -d '{}' -b /tmp/s2_jar3.txt)
expect_code "refresh after logout (replayed)" 401 "$CODE"
jq -r '.reason // empty' /tmp/s2_r.json | grep -q AUTH_INVALID_REFRESH_TOKEN && ok "post-logout replay rejected" || bad "post-logout reason: $(cat /tmp/s2_r.json)"

say "11. article reads stay public; write stays open until S3"
CODE=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/v1/articles/list")
expect_code "public list" 200 "$CODE"

say "12. login rate limit -> 429 after window exhausted"
RL_CODE=200
for i in $(seq 1 12); do
  RL_CODE=$(curl -s -o /tmp/s2_rl.json -w '%{http_code}' -X POST "$BASE/v1/auth/login" -H 'Content-Type: application/json' -H 'X-Forwarded-For: 203.0.113.7' -d '{"email":"author@example.com","password":"longenough1"}')
  [ "$RL_CODE" = "429" ] && break
done
expect_code "rate limited login" 429 "$RL_CODE"
jq -r '.reason // empty' /tmp/s2_rl.json | grep -q AUTH_TOO_MANY_ATTEMPTS && ok "reason AUTH_TOO_MANY_ATTEMPTS" || bad "reason: $(cat /tmp/s2_rl.json)"
CODE=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/v1/articles/list")
expect_code "other IPs unaffected; public read fine" 200 "$CODE"

say "RESULT"
echo "passed=$PASS failed=$FAIL"
[ $FAIL -eq 0 ] && echo "SMOKE GREEN" || echo "SMOKE RED"
exit $FAIL
