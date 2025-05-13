---
icon: material/package
---

# Package Manager

## :material-tram: Repository Installation

=== ":material-debian: Debian / APT"

    ```bash
    sudo mkdir -p /etc/apt/keyrings &&
       sudo curl -fsSL https://sing-box.app/gpg.key -o /etc/apt/keyrings/sagernet.asc &&
       sudo chmod a+r /etc/apt/keyrings/sagernet.asc &&
       echo '
    Types: deb
    URIs: https://deb.sagernet.org/
    Suites: *
    Components: *
    Enabled: yes
    Signed-By: /etc/apt/keyrings/sagernet.asc
    ' | sudo tee /etc/apt/sources.list.d/sagernet.sources &&
       sudo apt-get update &&
       sudo apt-get install sing-box # or sing-box-beta
    ```

=== ":material-redhat: Redhat / DNF 5"

    ```bash
    sudo dnf config-manager addrepo --from-repofile=https://sing-box.app/sing-box.repo &&
    sudo dnf install sing-box # or sing-box-beta
    ```

=== ":material-redhat: Redhat / DNF 4"

    ```bash
    sudo dnf config-manager --add-repo https://sing-box.app/sing-box.repo &&
    sudo dnf -y install dnf-plugins-core &&
    sudo dnf install sing-box # or sing-box-beta
    ```

## :material-download-box: Manual Installation

The script download and install the latest package from GitHub releases
for deb or rpm based Linux distributions, ArchLinux and OpenWrt.

```shell
curl -fsSL https://sing-box.app/install.sh | sh
```

or latest beta:

```shell
curl -fsSL https://sing-box.app/install.sh | sh -s -- --beta
```

or specific version:

```shell
curl -fsSL https://sing-box.app/install.sh | sh -s -- --version <version>
```

## :material-book-lock-open: Managed Installation

=== ":material-linux: Linux"

    | Type     | Platform      | Command                      | Link                                                                                                          |
    |----------|---------------|------------------------------|---------------------------------------------------------------------------------------------------------------|
    | AUR      | Arch Linux    | `? -S sing-box`              | [![AUR package](https://repology.org/badge/version-for-repo/aur/sing-box.svg)][aur]                           |
    | nixpkgs  | NixOS         | `nix-env -iA nixos.sing-box` | [![nixpkgs unstable package](https://repology.org/badge/version-for-repo/nix_unstable/sing-box.svg)][nixpkgs] |
    | Homebrew | macOS / Linux | `brew install sing-box`      | [![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/sing-box.svg)][brew]                |
    | APK      | Alpine        | `apk add sing-box`           | [![Alpine Linux Edge package](https://repology.org/badge/version-for-repo/alpine_edge/sing-box.svg)][alpine]  |
    | DEB      | AOSC          | `apt install sing-box`       | [![AOSC package](https://repology.org/badge/version-for-repo/aosc/sing-box.svg)][aosc]                        |

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

## :material-alert: Problematic Sources

| Type       | Platform | Link                                                                                      | Promblem(s)                             |
|------------|----------|-------------------------------------------------------------------------------------------|-----------------------------------------|
| DEB        | AOSC     | [aosc-os-abbs](https://github.com/AOSC-Dev/aosc-os-abbs/tree/stable/app-network/sing-box) | Problematic build tag list modification |
| Homebrew   | /        | [homebrew-core][brew]                                                                     | Problematic build tag list modification |
| Termux     | Android  | [termux-packages][termux]                                                                 | Problematic build tag list modification |
| FreshPorts | FreeBSD  | [FreeBSD ports][ports]                                                                    | Old Go  (go1.20)                        |

If you are a user of them, please report issues to them:

1. Please do not modify release build tags without full understanding of the related functionality: enabling non-default
   labels may result in decreased performance; the lack of default labels may cause user confusion.
2. sing-box supports compiling with some older Go versions, but it is not recommended (especially versions that are no
   longer supported by Go).

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

[aosc]: https://packages.aosc.io/packages/sing-box

[systemd]: https://systemd.io/
