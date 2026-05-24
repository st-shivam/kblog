#!/usr/bin/env python3
"""Run a command with a timeout, capture combined stdout+stderr, print it."""
import subprocess, sys, os

args = sys.argv[1:]  # first arg is timeout seconds, rest is the command
timeout = int(args[0])
cmd = args[1:]

try:
    r = subprocess.run(cmd, capture_output=True, timeout=timeout)
    sys.stdout.buffer.write(r.stdout)
    sys.stdout.buffer.write(r.stderr)
except subprocess.TimeoutExpired:
    pass  # timeout is expected for TUI processes
except Exception as e:
    print(str(e), file=sys.stderr)
