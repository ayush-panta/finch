#!/bin/bash
# Credential helper that forwards to host daemon via nc
LOGFILE="/tmp/cred-helper-debug.log"
echo "" >> $LOGFILE
echo "[$(date '+%%m/%%d %%l:%%M%%p')] Credential helper called with args: $@" >> $LOGFILE
echo "[$(date '+%%m/%%d %%l:%%M%%p')] Input received:" >> $LOGFILE
input=$(cat)
echo "$input" >> $LOGFILE
# echo "[$(date '+%%m/%%d %%l:%%M%%p')] Forwarding to host daemon via nc" >> $LOGFILE
# Use nc with timeout and proper response handling
{
    printf "%%s\n%%s\n" "$1" "$input"
    sleep 0.1
} | timeout 10 nc -U /tmp/finch-creds.sock > /tmp/nc_response 2>>$LOGFILE
exit_code=$?
response=$(cat /tmp/nc_response 2>/dev/null)
# echo "[$(date '+%%m/%%d %%l:%%M%%p')] NC exit code: $exit_code" >> $LOGFILE
echo "[$(date '+%%m/%%d %%l:%%M%%p')] Response from host: $response" >> $LOGFILE
if [ $exit_code -ne 0 ]; then
    echo "[$(date '+%%m/%%d %%l:%%M%%p')] Error: nc failed with exit code $exit_code" >> $LOGFILE
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
rm -f /tmp/nc_response