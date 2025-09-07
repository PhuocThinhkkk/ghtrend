#!/usr/bin/env bash

# This file is for linux


set -e

APP_NAME="ghtrend"

echo "░██████╗░██╗░░██╗████████╗██████╗░███████╗███╗░░██╗██████╗░"
echo "██╔════╝░██║░░██║╚══██╔══╝██╔══██╗██╔════╝████╗░██║██╔══██╗"
echo "██║░░██╗░███████║░░░██║░░░██████╔╝█████╗░░██╔██╗██║██║░░██║"
echo "██║░░╚██╗██╔══██║░░░██║░░░██╔══██╗██╔══╝░░██║╚████║██║░░██║"
echo "╚██████╔╝██║░░██║░░░██║░░░██║░░██║███████╗██║░╚███║██████╔╝"
echo "░╚═════╝░╚═╝░░╚═╝░░░╚═╝░░░╚═╝░░╚═╝╚══════╝╚═╝░░╚══╝╚═════╝░"

echo "Building $APP_NAME..."
go build -o "$APP_NAME" .

chmod +x "$APP_NAME"

INSTALL_DIR="/usr/local/bin"

if [ ! -w "$INSTALL_DIR" ]; then
  echo "You don't have write permission to $INSTALL_DIR, using ~/.local/bin instead."
  INSTALL_DIR="$HOME/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

echo "Installing $APP_NAME to $INSTALL_DIR"
mv "$APP_NAME" "$INSTALL_DIR/"

if command -v "$APP_NAME" >/dev/null 2>&1; then
  echo "$APP_NAME installed successfully! Run: $APP_NAME --help"
  echo -e "\033[32mThank you for using my app, this is my first cli project\033[0m"
else
  echo "Installed, but $INSTALL_DIR is not in your PATH."
  echo "Add this to your shell config (e.g. ~/.bashrc):"
  echo "    export PATH=\$PATH:$INSTALL_DIR"
fi


