#!/bin/bash
# Reader-accounts smoke (T-010/S1): register -> profile -> avatar upload ->
# avatar serving + negative cases. Usage: backend/scripts/smoke-account.sh
# Requires the dev compose stack (mysql :3307, redis :6379) up and jq.
set -u
cd "$(dirname "$0")/.."
BASE=${BASE:-http://127.0.0.1:8000}
PASS=0; FAIL=0
say() { printf '\n== %s\n' "$*"; }
ok()  { PASS=$((PASS+1)); echo "PASS: $*"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL: $*"; }
expect_code() { # desc expected actual
  if [ "$3" = "$2" ]; then ok "$1 (HTTP $3)"; else bad "$1 want $2 got $3"; fi
}

say "start server with a fresh secret"
export KRATOS_JWT_SECRET=$(openssl rand -hex 32)
export UPLOADS_DIR=$(mktemp -d)
# Dev-only: clear stale rate-limit windows from earlier runs.
docker exec blog-redis redis-cli FLUSHDB >/dev/null
go run ./cmd/server -conf ./configs >/tmp/s1_server.log 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null' EXIT
for i in $(seq 1 60); do
  curl -sf "$BASE/v1/articles/list" >/dev/null 2>&1 && break
  sleep 0.5
done

READER_EMAIL=reader-account-smoke@example.com
# Unique per run so re-runs never collide with a leftover row.
STAMP=$(date +%s)
READER_EMAIL="reader-$STAMP@example.com"
READER_PASSWORD=longenough1

say "1. register -> 200, READER role, auto signed-in with refresh cookie"
REG=$(curl -s -c /tmp/s1_jar.txt -D /tmp/s1_h.txt -X POST "$BASE/v1/auth/register" -H 'Content-Type: application/json' -d "{\"email\":\"$READER_EMAIL\",\"password\":\"$READER_PASSWORD\",\"display_name\":\"  冒烟读者 \"}")
ACCESS=$(echo "$REG" | jq -r '.access_token // empty')
[ -n "$ACCESS" ] && ok "access token issued" || bad "no access token: $REG"
echo "$REG" | jq -e '.user.role == 2 and .user.display_name == "冒烟读者" and .user.avatar_url == ""' >/dev/null && ok "reader role + trimmed name + empty avatar" || bad "user fields: $REG"
grep -i 'set-cookie: refresh_token=' /tmp/s1_h.txt >/dev/null && ok "refresh cookie set" || bad "no refresh cookie"

say "2. duplicate email (case-insensitive) -> 409"
UPPER=$(echo "$READER_EMAIL" | tr 'a-z' 'A-Z')
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/register" -H 'Content-Type: application/json' -d "{\"email\":\"$UPPER\",\"password\":\"$READER_PASSWORD\",\"display_name\":\"x\"}")
expect_code "duplicate email" 409 "$CODE"
jq -r '.reason // empty' /tmp/s1_r.json | grep -q USER_EMAIL_CONFLICT && ok "reason USER_EMAIL_CONFLICT" || bad "reason: $(cat /tmp/s1_r.json)"

say "3. weak password -> 400"
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/register" -H 'Content-Type: application/json' -d '{"email":"weak@example.com","password":"short","display_name":"x"}')
expect_code "weak password" 400 "$CODE"

say "4. blank display name -> 400"
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X POST "$BASE/v1/auth/register" -H 'Content-Type: application/json' -d '{"email":"blank@example.com","password":"longenough1","display_name":"   "}')
expect_code "blank display name" 400 "$CODE"

say "5. profile update without token -> 401"
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X PUT "$BASE/v1/user/profile" -H 'Content-Type: application/json' -d '{"display_name":"改名"}')
expect_code "profile without token" 401 "$CODE"

say "6. profile update with token -> 200, me reflects it"
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X PUT "$BASE/v1/user/profile" -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' -d '{"display_name":"改名读者"}')
expect_code "profile rename" 200 "$CODE"
ME=$(curl -s "$BASE/v1/auth/me" -H "Authorization: Bearer $ACCESS")
echo "$ME" | jq -e '.display_name == "改名读者"' >/dev/null && ok "me returns renamed account" || bad "me body: $ME"

say "7. avatar upload (real PNG) -> 200 with avatar_url"
PNG=$(python3 - <<'EOF'
import base64, struct, zlib
# Minimal 1x1 red PNG, built by hand so the smoke needs no fixtures.
def chunk(t, d):
    c = struct.pack('>I', len(d)) + t + d
    return c + struct.pack('>I', zlib.crc32(t + d) & 0xffffffff)
png = b'\x89PNG\r\n\x1a\n'
png += chunk(b'IHDR', struct.pack('>IIBBBBB', 1, 1, 8, 2, 0, 0, 0))
png += chunk(b'IDAT', zlib.compress(b'\x00\xff\x00\x00'))
png += chunk(b'IEND', b'')
print(base64.b64encode(png).decode())
EOF
)
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X PUT "$BASE/v1/user/avatar" -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' -d "{\"data\":\"$PNG\"}")
expect_code "avatar upload" 200 "$CODE"
AVATAR_URL=$(jq -r '.avatar_url // empty' /tmp/s1_r.json)
case "$AVATAR_URL" in
  /v1/assets/avatars/*) ok "avatar_url = $AVATAR_URL" ;;
  *) bad "avatar url: $(cat /tmp/s1_r.json)" ;;
esac

say "8. avatar content-type + cache headers; traversal rejected"
AVATAR_NAME=${AVATAR_URL##*/}
CODE=$(curl -s -o /tmp/s1_avatar.png -D /tmp/s1_ah.txt -w '%{http_code}' "$BASE/v1/assets/avatars/$AVATAR_NAME")
expect_code "avatar fetch" 200 "$CODE"
grep -i '^content-type: image/png' /tmp/s1_ah.txt >/dev/null && ok "content-type image/png" || bad "content-type: $(grep -i content-type /tmp/s1_ah.txt)"
grep -i 'cache-control' /tmp/s1_ah.txt | grep -q 'immutable' && ok "cache-control immutable" || bad "no cache-control"
cmp -s <(base64 -d <<<"$PNG") /tmp/s1_avatar.png && ok "served bytes identical" || bad "served bytes differ"
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' "$BASE/v1/assets/avatars/0123456789abcdef0123456789abcdef.txt")
expect_code "non-image extension" 404 "$CODE"
CODE=$(curl -s -o /dev/null -w '%{http_code}' --path-as-is "$BASE/v1/assets/avatars/..%2f..%2fsecrets.local.yaml")
expect_code "path traversal" 404 "$CODE"

say "9. avatar upload rejects junk content -> 400"
CODE=$(curl -s -o /tmp/s1_r.json -w '%{http_code}' -X PUT "$BASE/v1/user/avatar" -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' -d '{"data":"bm90IGFuIGltYWdl"}')
expect_code "text as avatar" 400 "$CODE"

say "10. register rate limit -> 429 on its own budget"
RL_CODE=200
for i in $(seq 1 12); do
  RL_CODE=$(curl -s -o /tmp/s1_rl.json -w '%{http_code}' -X POST "$BASE/v1/auth/register" -H 'Content-Type: application/json' -H 'X-Forwarded-For: 203.0.113.8' -d "{\"email\":\"rl-$STAMP-$i@example.com\",\"password\":\"longenough1\",\"display_name\":\"rl\"}")
  [ "$RL_CODE" = "429" ] && break
done
expect_code "rate limited register" 429 "$RL_CODE"
jq -r '.reason // empty' /tmp/s1_rl.json | grep -q AUTH_TOO_MANY_ATTEMPTS && ok "reason AUTH_TOO_MANY_ATTEMPTS" || bad "reason: $(cat /tmp/s1_rl.json)"

say "RESULT"
echo "passed=$PASS failed=$FAIL"
[ $FAIL -eq 0 ] && echo "SMOKE GREEN" || echo "SMOKE RED"
exit $FAIL
