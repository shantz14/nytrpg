#!/usr/bin/env bash
# One-time setup of a fresh EC2 instance (Amazon Linux 2023 or Ubuntu): Docker,
# the compose plugin, swap, and ~/nytrpg/.env. Run on the instance:
#   curl -fsSL https://raw.githubusercontent.com/shantz14/nytrpg/main/deploy/ec2-setup.sh | bash
# Then deploy from your machine with deploy/update.sh
set -euo pipefail

DIR=${DIR:-$HOME/nytrpg}

if command -v dnf >/dev/null; then
    sudo dnf install -y docker
    # Amazon Linux has no compose plugin package
    sudo mkdir -p /usr/local/lib/docker/cli-plugins
    sudo curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$(uname -m)" \
        -o /usr/local/lib/docker/cli-plugins/docker-compose
    sudo chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
elif command -v apt-get >/dev/null; then
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl
    curl -fsSL https://get.docker.com | sudo sh
else
    echo "unsupported distro" >&2
    exit 1
fi
sudo systemctl enable --now docker
sudo usermod -aG docker "$USER"

# A micro instance has 1GB of memory
if [ ! -f /swapfile ]; then
    sudo fallocate -l 1G /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
    echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
fi

mkdir -p "$DIR/db" "$DIR/deploy"
if [ ! -f "$DIR/.env" ]; then
    (umask 077; echo "JWT_SECRET=$(openssl rand -hex 32)" > "$DIR/.env")
fi

echo
echo "Done. Log out and back in (for the docker group), add ANTHROPIC_API_KEY to $DIR/.env"
echo "if you want Cleric prayers, then run deploy/update.sh from your machine."
