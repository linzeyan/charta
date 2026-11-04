#!/bin/bash
# Installs what `make check` needs on Ubuntu 24.04 beyond Rust, Go, mise and
# Chrome: the headers Gio and Fyne compile against (cgo), the ones Slint's
# and egui's windowing crates link, Mesa's software drivers so the native
# backends render without a GPU (lavapipe for egui's wgpu, llvmpipe behind
# Gio's EGL), and the fonts the system-font backends set text in.
#
# Usage: sudo scripts/linux-deps.sh   (CI runs it; so does docs/scripts/ci-linux.sh)

export DEBIAN_FRONTEND=noninteractive
apt-get update -q || exit 1
apt-get install -yq --no-install-recommends \
  pkg-config \
  libegl1-mesa-dev libgles2-mesa-dev libgl1-mesa-dev libvulkan-dev \
  libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-dev libxkbcommon-x11-dev \
  libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev libxfixes-dev \
  libfontconfig1-dev \
  mesa-vulkan-drivers libgl1-mesa-dri libegl-mesa0 \
  fonts-dejavu-core fonts-noto-core fonts-noto-cjk
