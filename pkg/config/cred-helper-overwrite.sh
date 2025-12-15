#!/bin/bash
# Credential helper that forwards to host daemon via socat
LOGFILE="/tmp/cred-helper-debug.log"
echo "" >> $LOGFILE
echo "[$(date '+%%m/%%d %%l:%%M%%p')] Credential helper called with args: $@" >> $LOGFILE
echo "[$(date '+%%m/%%d %%l:%%M%%p')] Input received:" >> $LOGFILE
input=$(cat)
echo "$input" >> $LOGFILE

# Forward to host daemon via socat
response=$(printf "%s\n%s\n" "$1" "$input" | socat - UNIX-CONNECT:/tmp/finch-creds.sock 2>>$LOGFILE)
exit_code=$?

echo "[$(date '+%%m/%%d %%l:%%M%%p')] Response from host: $response" >> $LOGFILE

if [ $exit_code -ne 0 ]; then
    echo "[$(date '+%%m/%%d %%l:%%M%%p')] Error: socat failed with exit code $exit_code" >> $LOGFILE
    echo '{"error": "credential helper connection failed"}'
    exit 1
fi

# Handle empty response - only exit with code 1 for 'get' command when credentials not found
if [ -z "$response" ] && [ "$1" = "get" ]; then
    echo "[$(date '+%%m/%%d %%l:%%M%%p')] Empty response for get - credentials not found" >> $LOGFILE
    exit 1
fi

echo "$response"
echo "" >> $LOGFILE