---
icon: material/apple
---

# sing-box for Apple platforms

SFI/SFM/SFT allows users to manage and run local or remote sing-box configuration files, and provides
platform-specific function implementation, such as TUN transparent proxy implementation.

## :material-graph: Requirements

* iOS 15.0+ / macOS 13.0+ / Apple tvOS 17.0+
* An Apple account outside of mainland China

## :material-download: Download

* [App Store](https://apps.apple.com/app/sing-box-mt/id6785326793) (iOS / Apple tvOS)
* TestFlight (Beta)

TestFlight quota is only available to [sponsors](https://github.com/sponsors/nekohasekai)
(one-time sponsorships are accepted).
Once you donate, you can get an invitation by join our Telegram group for sponsors from [@yet_another_sponsor_bot](https://t.me/yet_another_sponsor_bot)
or sending us your Apple ID [via email](mailto:contact@sagernet.org).

## :material-cellphone-arrow-down: Download (iOS jailbreak version)

* [GitHub Releases](https://github.com/SagerNet/sing-box/releases)
    * `SFI-iphoneos-arm64.deb`: rootless
    * `SFI-iphoneos-arm64e.deb`: roothide

The jailbroken version of SFI: requires rootless or roothide iOS 15.0+

Additional features:

* It can run a [Tailscale SSH server](/configuration/endpoint/tailscale/#ssh_server) on the device.
* [Process matching](/configuration/route/rule/#process_name) (`process_name`, `process_path`, `user`, and so on) works in route and DNS rules.

## :material-file-download: Download (macOS standalone version)

* [Homebrew Cask](https://formulae.brew.sh/cask/sfm)

```bash
# brew install sfm
```

* [GitHub Releases](https://github.com/SagerNet/sing-box/releases)

## :material-source-repository: Source code

* [GitHub](https://github.com/SagerNet/sing-box-for-apple)
