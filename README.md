# sing-box

The `lantern-main` branch is our primary working branch. It will automatically be synced with `SagerNet/sing-box/main` on a weekly basis, but you can also trigger a manual sync by running the auto sync workflow.

## 

The universal proxy platform.

[![Packaging status](https://repology.org/badge/vertical-allrepos/sing-box.svg)](https://repology.org/project/sing-box/versions)

## Documentation

https://sing-box.sagernet.org

## License

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

In addition, no derivative work may use the name or imply association
with this application without prior consent.
```

To sync with the latest sing-box mainline, simply run:

```
git fetch upstream
git merge upstream/main
git push origin lantern-main-next
```

To make updating other repos easier, you can then do, for example:

```
git tag -a v1.11.11-lantern -m "tagging latest"
```
