#!/bin/bash
# Signed debug session with watch: stages the firmware and native helpers as
# build.sh does, builds the signed debug app, launches it through
# LaunchServices, and rebuilds and restarts it when Go sources or
# configuration change. Swift/Rust helper changes need a new session.
# MYGO_SIGN_IDENTITY selects the signing identity (default: ad hoc).
set -euo pipefail
exec "$(dirname "$0")/build.sh" --dev
