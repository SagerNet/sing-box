#!/bin/sh

download_beta=false
download_version=""

while [ $# -gt 0 ]; do
  case "$1" in
    --beta)
      download_beta=true
      shift
      ;;
    --version)
      shift
      if [ $# -eq 0 ]; then
        echo "Missing argument for --version"
        echo "Usage: $0 [--beta] [--version <version>]"
        exit 1
      fi
      download_version="$1"
      shift
      ;;
    *)
      echo "Unknown argument: $1"
      echo "Usage: $0 [--beta] [--version <version>]"
      exit 1
      ;;
  esac
done

if command -v pacman >/dev/null 2>&1; then
  os="linux"
  arch=$(uname -m)
  package_suffix=".pkg.tar.zst"
  package_install="pacman -U --noconfirm"
elif command -v dpkg >/dev/null 2>&1; then
  os="linux"
  arch=$(dpkg --print-architecture)
  package_suffix=".deb"
  package_install="dpkg -i"
elif command -v dnf >/dev/null 2>&1; then
  os="linux"
  arch=$(uname -m)
  package_suffix=".rpm"
  package_install="dnf install -y"
elif command -v rpm >/dev/null 2>&1; then
  os="linux"
  arch=$(uname -m)
  package_suffix=".rpm"
  package_install="rpm -i"
elif command -v opkg >/dev/null 2>&1; then
  os="openwrt"
  . /etc/os-release
  arch="$OPENWRT_ARCH"
  package_suffix=".ipk"
  package_install="opkg update && opkg install"
else
  echo "Missing supported package manager."
  exit 1
fi

if [ -z "$download_version" ]; then
  if [ "$download_beta" != "true" ]; then
    if [ -n "$GITHUB_TOKEN" ]; then
      latest_release=$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/SagerNet/sing-box/releases/latest)
    else
      latest_release=$(curl -s https://api.github.com/repos/SagerNet/sing-box/releases/latest)
    fi
    curl_exit_status=$?
    if [ $curl_exit_status -ne 0 ]; then
      exit $curl_exit_status
    fi
    if [ "$(echo "$latest_release" | grep tag_name | wc -l)" -eq 0 ]; then
      echo "$latest_release"
      exit 1
    fi
    download_version=$(echo "$latest_release" | grep tag_name | head -n 1 | awk -F: '{print $2}' | sed 's/[", v]//g')
  else
    if [ -n "$GITHUB_TOKEN" ]; then
      latest_release=$(curl -s -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/SagerNet/sing-box/releases)
    else
      latest_release=$(curl -s https://api.github.com/repos/SagerNet/sing-box/releases)
    fi
    curl_exit_status=$?
    if [ $curl_exit_status -ne 0 ]; then
      exit $curl_exit_status
    fi
    if [ "$(echo "$latest_release" | grep tag_name | wc -l)" -eq 0 ]; then
      echo "$latest_release"
      exit 1
    fi
    download_version=$(echo "$latest_release" | grep tag_name | head -n 1 | awk -F: '{print $2}' | sed 's/[", v]//g')
  fi
fi

package_name="sing-box_${download_version}_${os}_${arch}${package_suffix}"
package_url="https://github.com/SagerNet/sing-box/releases/download/v${download_version}/${package_name}"

echo "Downloading $package_url"
if [ -n "$GITHUB_TOKEN" ]; then
  curl --fail -Lo "$package_name" -H "Authorization: token ${GITHUB_TOKEN}" "$package_url"
else
  curl --fail -Lo "$package_name" "$package_url"
fi

curl_exit_status=$?
if [ $curl_exit_status -ne 0 ]; then
  exit $curl_exit_status
fi

if command -v sudo >/dev/null 2>&1; then
  package_install="sudo $package_install"
fi

echo "$package_install $package_name"
sh -c "$package_install \"$package_name\""
rm -f "$package_name"
