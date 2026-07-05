#!/usr/bin/env bash
set -euo pipefail

MSL_BIN="${1:-./bin/msl}"
ROOT="$(mktemp -d)"
GIT_ROOT=""
trap 'rm -rf "$ROOT" "$GIT_ROOT"' EXIT

export MSL_CONFIG="$ROOT/config.toml"
SCRIPTS_DIR="$ROOT/scripts"
SHIM_DIR="$ROOT/bin"

"$MSL_BIN" init --dir "$SCRIPTS_DIR" --shims "$SHIM_DIR" --no-git --yes >/tmp/msl-init.out
grep -q "Script library ready" /tmp/msl-init.out

cat >"$SCRIPTS_DIR/hello" <<'SCRIPT'
#!/usr/bin/env bash
# msl:name Hello
# msl:description Print a friendly greeting.
# msl:usage hello [name]
# msl:tags demo, greeting
# msl:runtime bash
# msl:safety read-only
# msl:deps bash
# msl:example hello Codex

name="${1:-world}"
printf 'hello %s\n' "$name"
SCRIPT
chmod +x "$SCRIPTS_DIR/hello"
"$MSL_BIN" scripts shims | grep -q "Synced 1 shims"
test -L "$SHIM_DIR/hello"

"$MSL_BIN" search greeting | grep -q "hello"
"$MSL_BIN" info hello | grep -q "Safety: read-only"
test "$("$MSL_BIN" run hello -- Codex)" = "hello Codex"
test "$("$MSL_BIN" hello Codex)" = "hello Codex"
"$MSL_BIN" validate "$SCRIPTS_DIR/hello" | grep -q "Validation passed"

PROVIDER="$ROOT/provider.sh"
cat >"$PROVIDER" <<'SCRIPT'
#!/usr/bin/env bash
cat <<'GENERATED'
#!/usr/bin/env bash
# msl:name Say Hi
# msl:description Say hi to the current user.
# msl:usage say-hi
# msl:tags demo, greeting
# msl:runtime bash
# msl:safety read-only
# msl:deps bash
# msl:example say-hi

printf 'hi\n'
GENERATED
SCRIPT
chmod +x "$PROVIDER"
MSL_LLM_CUSTOM_COMMAND="$PROVIDER" "$MSL_BIN" new --yes "say hi" | grep -q "Saved:"
test -x "$SCRIPTS_DIR/say-hi"

"$MSL_BIN" mcp tools | grep -q "search_scripts"
"$MSL_BIN" mcp instructions | grep -q "Before creating reusable local utility scripts"

"$MSL_BIN" validate scripts/find-pid-by-port | grep -q "Validation passed"
"$MSL_BIN" validate scripts/kill-by-port | grep -q "Validation passed"

PORT=$((30000 + ($$ % 10000)))
python3 -m http.server "$PORT" --bind 127.0.0.1 >/tmp/msl-port-server.out 2>&1 &
SERVER_PID=$!
cleanup_port_server() {
  kill "$SERVER_PID" >/dev/null 2>&1 || true
}
trap 'cleanup_port_server; rm -rf "$ROOT" "$GIT_ROOT"' EXIT
for _ in 1 2 3 4 5; do
  if scripts/find-pid-by-port "$PORT" 2>/dev/null | grep -q "pid=$SERVER_PID"; then
    break
  fi
  sleep 0.2
done
scripts/find-pid-by-port "$PORT" | grep -q "pid=$SERVER_PID"
scripts/kill-by-port "$PORT" --dry-run | grep -q "Dry run"
scripts/kill-by-port "$PORT" --yes | grep -q "Sent signal"
sleep 0.2
if kill -0 "$SERVER_PID" >/dev/null 2>&1; then
  echo "port server was not killed" >&2
  exit 1
fi

GIT_ROOT="$(mktemp -d)"
export MSL_CONFIG="$GIT_ROOT/config.toml"
GIT_SCRIPTS="$GIT_ROOT/scripts"
"$MSL_BIN" init --dir "$GIT_SCRIPTS" --yes >/tmp/msl-git-init.out
MSL_LLM_CUSTOM_COMMAND="$PROVIDER" "$MSL_BIN" new --yes "say hi with git" >/tmp/msl-git-new.out
grep -q "Committed: Add say-hi script" /tmp/msl-git-new.out
STATUS="$("$MSL_BIN" scripts status)"
test -z "$STATUS"
"$MSL_BIN" scripts log say-hi | grep -q "Add say-hi script"

echo "e2e passed"
