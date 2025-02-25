---
icon: material/package
---

# 包管理器

## :material-tram: 仓库安装

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

## :material-download-box: 手动安装

该脚本从 GitHub 发布中下载并安装最新的软件包，适用于基于 deb 或 rpm 的 Linux 发行版、ArchLinux 和 OpenWrt。

```shell
curl -fsSL https://sing-box.app/install.sh | sh
```

或最新测试版：

```shell
curl -fsSL https://sing-box.app/install.sh | sh -s -- --beta
```

或指定版本：

```shell
curl -fsSL https://sing-box.app/install.sh | sh -s -- --version <version>
```

## :material-book-lock-open: 托管安装

=== ":material-linux: Linux"

    | 类型       | 平台            | 命令                           | 链接                                                                                                            |
    |----------|---------------|------------------------------|---------------------------------------------------------------------------------------------------------------|
    | AUR      | Arch Linux    | `? -S sing-box`              | [![AUR package](https://repology.org/badge/version-for-repo/aur/sing-box.svg)][aur]                           |
    | nixpkgs  | NixOS         | `nix-env -iA nixos.sing-box` | [![nixpkgs unstable package](https://repology.org/badge/version-for-repo/nix_unstable/sing-box.svg)][nixpkgs] |
    | Homebrew | macOS / Linux | `brew install sing-box`      | [![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/sing-box.svg)][brew]                |
    | APK      | Alpine        | `apk add sing-box`           | [![Alpine Linux Edge package](https://repology.org/badge/version-for-repo/alpine_edge/sing-box.svg)][alpine]  |
    | DEB      | AOSC          | `apt install sing-box`       | [![AOSC package](https://repology.org/badge/version-for-repo/aosc/sing-box.svg)][aosc]                        |

=== ":material-apple: macOS"

    | 类型       | 平台    | 命令                      | 链接                                                                                             |
    |----------|-------|-------------------------|------------------------------------------------------------------------------------------------|
    | Homebrew | macOS | `brew install sing-box` | [![Homebrew package](https://repology.org/badge/version-for-repo/homebrew/sing-box.svg)][brew] |

=== ":material-microsoft-windows: Windows"

    | 类型         | 平台      | 命令                        | 链接                                                                                                  |
    |------------|---------|---------------------------|-----------------------------------------------------------------------------------------------------|
    | Scoop      | Windows | `scoop install sing-box`  | [![Scoop package](https://repology.org/badge/version-for-repo/scoop/sing-box.svg)][scoop]           |
    | Chocolatey | Windows | `choco install sing-box`  | [![Chocolatey package](https://repology.org/badge/version-for-repo/chocolatey/sing-box.svg)][choco] |
    | winget     | Windows | `winget install sing-box` | [![winget package](https://repology.org/badge/version-for-repo/winget/sing-box.svg)][winget]        |

=== ":material-android: Android"

    | 类型     | 平台      | 命令                 | 链接                                                                                           |
    |--------|---------|--------------------|----------------------------------------------------------------------------------------------|
    | Termux | Android | `pkg add sing-box` | [![Termux package](https://repology.org/badge/version-for-repo/termux/sing-box.svg)][termux] |

=== ":material-freebsd: FreeBSD"

    | 类型         | 平台      | 命令                     | 链接                                                                                         |
    |------------|---------|------------------------|--------------------------------------------------------------------------------------------|
    | FreshPorts | FreeBSD | `pkg install sing-box` | [![FreeBSD port](https://repology.org/badge/version-for-repo/freebsd/sing-box.svg)][ports] |

## :material-alert: 存在问题的源

| 类型         | 平台      | 链接                                                                                        | 原因              |
|------------|---------|-------------------------------------------------------------------------------------------|-----------------|
| DEB        | AOSC    | [aosc-os-abbs](https://github.com/AOSC-Dev/aosc-os-abbs/tree/stable/app-network/sing-box) | 存在问题的构建标志列表修改   |
| Homebrew   | /       | [homebrew-core][brew]                                                                     | 存在问题的构建标志列表修改   |
| Termux     | Android | [termux-packages][termux]                                                                 | 存在问题的构建标志列表修改   |
| FreshPorts | FreeBSD | [FreeBSD ports][ports]                                                                    | 太旧的 Go (go1.20) |

如果您是其用户，请向他们报告问题：

1. 在未完全了解相关功能的情况下，请勿修改发布版本标签：启用非默认标签可能会导致性能下降；缺少默认标签可能会引起用户混淆。
2. sing-box 支持使用一些较旧的 Go 版本进行编译，但不推荐使用（特别是已不再受 Go 支持的版本）。

## :material-book-multiple: 服务管理

对于带有 [systemd][systemd] 的 Linux 系统，通常安装已经包含 sing-box 服务，
您可以使用以下命令管理服务：

| 行动   | 命令                                            |
|------|-----------------------------------------------|
| 启用   | `sudo systemctl enable sing-box`              |
| 禁用   | `sudo systemctl disable sing-box`             |
| 启动   | `sudo systemctl start sing-box`               |
| 停止   | `sudo systemctl stop sing-box`                |
| 强行停止 | `sudo systemctl kill sing-box`                |
| 重新启动 | `sudo systemctl restart sing-box`             |
| 查看日志 | `sudo journalctl -u sing-box --output cat -e` |
| 实时日志 | `sudo journalctl -u sing-box --output cat -f` |

[alpine]: https://pkgs.alpinelinux.org/packages?name=sing-box

[aur]: https://aur.archlinux.org/packages/sing-box

[nixpkgs]: https://github.com/NixOS/nixpkgs/blob/nixos-unstable/pkgs/tools/networking/sing-box/default.nix

[brew]: https://formulae.brew.sh/formula/sing-box

[choco]: https://chocolatey.org/packages/sing-box

[scoop]: https://github.com/ScoopInstaller/Main/blob/master/bucket/sing-box.json

[winget]: https://github.com/microsoft/winget-pkgs/tree/master/manifests/s/SagerNet/sing-box

[termux]: https://github.com/termux/termux-packages/tree/master/packages/sing-box

[ports]: https://www.freshports.org/net/sing-box

[aosc]: https://packages.aosc.io/packages/sing-box

[systemd]: https://systemd.io/
