# Development environment

#### For the documentation

##### Setup

You need to configure python3 and pip first.

```shell
pip install mkdocs-material mkdocs-static-i18n
```

##### Run the site locally

```shell
mkdocs serve
```

or

```shell
python3 -m mkdocs serve
```

#### For the project

By default you have the latest Go installed (currently 1.19), and added `GOPATH/bin` to the PATH environment variable.

##### Setup

```shell
make fmt_insall
make lint_install
```

This installs the formatting and lint tools, which can be used via `make fmt` and `make lint`.

For ProtoBuffer changes, you also need `make proto_install` and `make proto`.

##### Build binary to the project directory

```shell
make
```

##### Install binary to GOPATH/bin

```shell
make install
```