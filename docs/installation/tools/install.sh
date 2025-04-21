#!/bin/sh

download_beta=false
download_version=""

for arg in "$@"; do
  if [[ "$arg" == "--beta" ]]; then
    download_beta=true
  elif [[ "$arg" == "--version" ]]; then
    download_version=true
  elif [[ "$download_version" == 'true' ]]; then
    download_version="$arg"
  else
    echo "Unknown argument: $arg"
    echo "Usage: $0 [--beta] [--version <version>]"
    exit 1
  fi
done

if [[ $(command -v dpkg) ]]; then
  os="linux"
  arch=$(dpkg --print-architecture)
  package_suffix=".deb"
  package_install="dpkg -i"
elif [[ $(command -v dnf) ]]; then
  os="linux"
  arch=$(uname -m)
  package_suffix=".rpm"
  package_install="dnf install -y"
elif [[ $(command -v rpm) ]]; then
  os="linux"
  arch=$(uname -m)
  package_suffix=".rpm"
  package_install="rpm -i"
elif [[ $(command -v pacman) ]]; then
  os="linux"
  arch=$(uname -m)
  package_suffix=".pkg.tar.zst"
  package_install="pacman -U --noconfirm"
elif [[ $(command -v opkg) ]]; then
  os="openwrt"
  source /etc/os-release
  arch="$OPENWRT_ARCH"
  package_suffix=".ipk"
  package_install="opkg update && opkg install -y"
else
  echo "Missing supported package manager."
  exit 1
fi

if [[ -z "$download_version" ]]; then
  if [[ "$download_beta" != 'true' ]]; then
    if [[ -n "$GITHUB_TOKEN" ]]; then
      latest_release=$(curl -s --fail-with-body -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/SagerNet/sing-box/releases/latest)
    else
      latest_release=$(curl -s --fail-with-body https://api.github.com/repos/SagerNet/sing-box/releases/latest)
    fi
    curl_exit_status=$?
    if [[ $curl_exit_status -ne 0 ]]; then
      echo "$latest_release"
      exit $?
    fi
    download_version=$(echo "$latest_release" | grep tag_name | cut -d ":" -f2 | sed 's/\"//g;s/\,//g;s/\ //g;s/v//')
  else
    if [[ -n "$GITHUB_TOKEN" ]]; then
      latest_release=$(curl -s --fail-with-body -H "Authorization: token ${GITHUB_TOKEN}" https://api.github.com/repos/SagerNet/sing-box/releases)
    else
      latest_release=$(curl -s --fail-with-body https://api.github.com/repos/SagerNet/sing-box/releases)
    fi
    curl_exit_status=$?
    if [[ $? -ne 0 ]]; then
      echo "$latest_release"
      exit $?
    fi
    download_version=$(echo "$latest_release" | grep tag_name | head -n 1 | cut -d ":" -f2 | sed 's/\"//g;s/\,//g;s/\ //g;s/v//')
  fi
fi

package_name="sing-box_${download_version}_${os}_${arch}${package_suffix}"
package_url="https://github.com/SagerNet/sing-box/releases/download/v${download_version}/${package_name}"

echo "Downloading $package_url"
if [[ -n "$GITHUB_TOKEN" ]]; then
  curl --fail-with-body -Lo "$package_name" -H "Authorization: token ${GITHUB_TOKEN}" "$package_url"
else
  curl --fail-with-body -Lo "$package_name" "$package_url"
fi

if [[ $? -ne 0 ]]; then
  exit $?
fi

if [[ $(command -v sudo) ]]; then
  package_install="sudo $package_install"
fi

echo "$package_install $package_name" && $package_install "$package_name" && rm "$package_name"
