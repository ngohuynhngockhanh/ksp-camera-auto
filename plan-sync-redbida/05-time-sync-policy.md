# Time Policy

The UI exposes host clock and NTP trust status through
`GET /api/redbida/time-status`.

The policy mirrors the existing KSP-Bida watchdog: host time must be NTP
trusted and drift threshold is 60 seconds. This first implementation does not
run `date -s`, does not mutate Node-RED, and does not add a privileged OS clock
write path.
