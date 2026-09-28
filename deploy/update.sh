#!/usr/bin/env bash
# Builds the image here and ships it to the instance (a micro instance has
# too little memory and disk to build it), then restarts the game:
#   EC2_HOST=ec2-user@1.2.3.4 deploy/update.sh
# Set EC2_KEY if the key isn't ~/.ssh/nytrpg-ec2-pair.pem
set -euo pipefail

: "${EC2_HOST:?set EC2_HOST, e.g. ec2-user@1.2.3.4}"
KEY=${EC2_KEY:-$HOME/.ssh/nytrpg-ec2-pair.pem}
cd "$(dirname "$0")/.."

docker build --platform linux/amd64 -t nytrpg:latest .
ssh -i "$KEY" "$EC2_HOST" 'mkdir -p ~/nytrpg/deploy ~/nytrpg/db'
scp -i "$KEY" docker-compose.yml "$EC2_HOST:nytrpg/"
scp -i "$KEY" deploy/Caddyfile "$EC2_HOST:nytrpg/deploy/"
docker save nytrpg:latest | gzip | ssh -i "$KEY" "$EC2_HOST" 'gunzip | docker load'
ssh -i "$KEY" "$EC2_HOST" 'set -e
    cd ~/nytrpg
    docker compose up -d
    docker image prune -f >/dev/null
    docker compose ps'
