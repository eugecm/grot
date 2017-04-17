# Grot: Simple text stream rotation

## Usage
Dump output logs to /var/log, rotate every 20 seconds, up to 5 files
> >/bin/endless_output | grot -to /var/log/endless.log -every 20s -max 5
