#!/bin/bash
set -euo pipefail

[ -s /root/.claude.json ] || echo '{}' > /root/.claude.json

command -v claude &>/dev/null || curl -fsSL https://claude.ai/install.sh | bash

npm install -g baedal
