#!/usr/bin/env bash
# One-time setup of a fresh EC2 instance (Amazon Linux 2023 or Ubuntu).
# Run on the instance:
#   curl -fsSL https://raw.githubusercontent.com/shantz14/nytrpg/main/deploy/ec2-setup.sh | bash
# Then edit ~/nytrpg/.env and run: cd ~/nytrpg && docker compose up -d --build
set -euo pipefail

REPO=${REPO:-https://github.com/shantz14/nytrpg.git}
DIR=${DIR:-$HOME/nytrpg}

if command -v dnf >/dev/null; then
    sudo dnf install -y docker git
    # Amazon Linux has no compose plugin package
    sudo mkdir -p /usr/local/lib/docker/cli-plugins
    sudo curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$(uname -m)" \
        -o /usr/local/lib/docker/cli-plugins/docker-compose
    sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
elif command -v apt-get >/dev/null; then
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl git
    curl -fsSL https://get.docker.com | sudo sh
else
    echo "unsupported distro" >&2
    exit 1
fi
sudo systemctl enable --now docker
sudo usermod -aG docker "$USER"

# Building the Go server needs more than the 1GB a micro instance has
if [ ! -f /swapfile ]; then
    sudo fallocate -l 2G /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
    echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
fi

if [ ! -d "$DIR" ]; then
    git clone "$REPO" "$DIR"
fi
cd "$DIR"
mkdir -p db
if [ ! -f .env ]; then
    cp .env.example .env
    sed -i "s/^JWT_SECRET=.*/JWT_SECRET=$(openssl rand -hex 32)/" .env
fi

echo
echo "Done. Log out and back in (for the docker group), then:"
echo "  cd $DIR && nano .env      # set SITE_ADDRESS and ANTHROPIC_API_KEY"
echo "  docker compose up -d --build"
