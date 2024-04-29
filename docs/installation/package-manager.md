---
icon: material/package
---

# Package Manager

## :material-tram: Repository Installation

=== ":material-debian: Debian / APT"

    ```bash
    sudo curl -fsSL https://sing-box.app/gpg.key -o /etc/apt/keyrings/sagernet.asc
    sudo chmod a+r /etc/apt/keyrings/sagernet.asc
    echo "deb [arch=`dpkg --print-architecture` signed-by=/etc/apt/keyrings/sagernet.asc] https://deb.sagernet.org/ * *" | \
      sudo tee /etc/apt/sources.list.d/sagernet.list > /dev/null
    sudo apt-get update
    sudo apt-get install sing-box # or sing-box-beta
    ```

=== ":material-redhat: Redhat / DNF"

    ```bash
    sudo dnf -y install dnf-plugins-core
    sudo dnf config-manager --add-repo https://sing-box.app/sing-box.repo
    sudo dnf install sing-box # or sing-box-beta
    ```

=== ":material-redhat: CentOS / YUM"

    ```bash
    sudo yum install -y yum-utils
    sudo yum-config-manager --add-repo https://sing-box.app/sing-box.repo
    sudo yum install sing-box # or sing-box-beta
    ```

## :material-download-box: Manual Installation

=== ":material-debian: Debian / DEB"

    ```bash
    bash <(curl -fsSL https://sing-box.app/deb-install.sh)
    ```

=== ":material-redhat: Redhat / RPM"

    ```bash
    bash <(curl -fsSL https://sing-box.app/rpm-install.sh)
    ```

=== ":simple-archlinux: Archlinux / PKG"

    ```bash
    bash <(curl -fsSL https://sing-box.app/arch-install.sh)
    ```

## :material-book-lock-open: Managed Installation

=== ":material-linux: Linux"

    | Type     | Platform      | Command                      | Link                                                                                                          |
    |----------|---------------|------------------------------|---------------------------------------------------------------------------------------------------------------|
    | AUR      | Arch Linux    | `? -S sing-box`              | [![AUR package](https://repology.org/badge/version-for-repo/aur/sing-box.svg)][aur]                           |
    | nixpkgs  | NixOS         | `nix-env -iA nixos.sing-box` | [![nixpkgs unstable package](https://repology.org/badge/version-for-repo/nix_unstable/sing-box.svg)][nixpkgs] |
    | Homebrew | macOS / Linux | `brew install sing-box`      | [![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/sing-box.svg)][brew]                |
    | APK      | Alpine        | `apk add sing-box`           | [![Alpine Linux Edge package](https://repology.org/badge/version-for-repo/alpine_edge/sing-box.svg)][alpine]  |

=== ":material-apple: macOS"

    | Type     | Platform | Command                 | Link                                                                                           |
    |----------|----------|-------------------------|------------------------------------------------------------------------------------------------|
    | Homebrew | macOS    | `brew install sing-box` | [![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/sing-box.svg)][brew] |

=== ":material-microsoft-windows: Windows"

    | Type       | Platform | Command                   | Link                                                                                                |
    |------------|----------|---------------------------|-----------------------------------------------------------------------------------------------------|
    | Scoop      | Windows  | `scoop install sing-box`  | [![Scoop package](https://repology.org/badge/version-for-repo/scoop/sing-box.svg)][scoop]           |
    | Chocolatey | Windows  | `choco install sing-box`  | [![Chocolatey package](https://repology.org/badge/version-for-repo/chocolatey/sing-box.svg)][choco] |
    | winget     | Windows  | `winget install sing-box` | [![winget package](https://repology.org/badge/version-for-repo/winget/sing-box.svg)][winget]        |

=== ":material-android: Android"

    | Type   | Platform | Command            | Link                                                                                         |
    |--------|----------|--------------------|----------------------------------------------------------------------------------------------|
    | Termux | Android  | `pkg add sing-box` | [![Termux package](https://repology.org/badge/version-for-repo/termux/sing-box.svg)][termux] |

=== ":material-freebsd: FreeBSD"

    | Type       | Platform | Command                | Link                                                                                       |
    |------------|----------|------------------------|--------------------------------------------------------------------------------------------|
    | FreshPorts | FreeBSD  | `pkg install sing-box` | [![FreeBSD port](https://repology.org/badge/version-for-repo/freebsd/sing-box.svg)][ports] |

## :material-book-multiple: Service Management

For Linux systems with [systemd][systemd], usually the installation already includes a sing-box service,
you can manage the service using the following command:

| Operation | Command                                       |
|-----------|-----------------------------------------------|
| Enable    | `sudo systemctl enable sing-box`              |
| Disable   | `sudo systemctl disable sing-box`             |
| Start     | `sudo systemctl start sing-box`               |
| Stop      | `sudo systemctl stop sing-box`                |
| Kill      | `sudo systemctl kill sing-box`                |
| Restart   | `sudo systemctl restart sing-box`             |
| Logs      | `sudo journalctl -u sing-box --output cat -e` |
| New Logs  | `sudo journalctl -u sing-box --output cat -f` |

[alpine]: https://pkgs.alpinelinux.org/packages?name=sing-box

[aur]: https://aur.archlinux.org/packages/sing-box

[nixpkgs]: https://github.com/NixOS/nixpkgs/blob/nixos-unstable/pkgs/tools/networking/sing-box/default.nix

[brew]: https://formulae.brew.sh/formula/sing-box

[openwrt]: https://github.com/openwrt/packages/tree/master/net/sing-box

[immortalwrt]: https://github.com/immortalwrt/packages/tree/master/net/sing-box

[choco]: https://chocolatey.org/packages/sing-box

[scoop]: https://github.com/ScoopInstaller/Main/blob/master/bucket/sing-box.json

[winget]: https://github.com/microsoft/winget-pkgs/tree/master/manifests/s/SagerNet/sing-box

[termux]: https://github.com/termux/termux-packages/tree/master/packages/sing-box

[ports]: https://www.freshports.org/net/sing-box

[systemd]: https://systemd.io/
