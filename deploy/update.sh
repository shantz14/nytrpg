#!/usr/bin/env bash
# Deploys the latest main to the instance from your machine:
#   EC2_HOST=ec2-user@1.2.3.4 deploy/update.sh
# Set EC2_KEY if the key isn't ~/.ssh/nytrpg-ec2-pair.pem
set -euo pipefail

: "${EC2_HOST:?set EC2_HOST, e.g. ec2-user@1.2.3.4}"
KEY=${EC2_KEY:-$HOME/.ssh/nytrpg-ec2-pair.pem}

ssh -i "$KEY" "$EC2_HOST" 'set -e
    cd ~/nytrpg
    git pull --ff-only
    docker compose up -d --build
    docker image prune -f >/dev/null
    docker compose ps'
