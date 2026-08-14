# ompphp

`ompphp` lets you write open.mp gamemodes in PHP. It embeds [Goro](https://github.com/KarpelesLab/goro) in the server component, so PHP code runs in-process and a separate PHP installation is not needed on the production server.

The PHP SDK covers the open.mp API and its events, with bindings generated from the official C API metadata.

## Installation

Download the component archive for your platform from the GitHub release, then copy the
`ompphp.so` or `ompphp.dll` file from its `components` directory into your
server's `components` directory. The open.mp `$CAPI` component must be installed
alongside it.

ompphp targets the 64-bit Linux and Windows server builds. It is not compatible
with the older 32-bit releases.

To build it yourself, clone the repository with its submodules and run the
matching task:

```bash
git clone --recurse-submodules https://github.com/ompphp/ompphp.git
cd ompphp
task component          # Linux
task component:windows  # Windows x64 cross-build from Linux
```

Source builds are written to `build/`.

Download `ompphp-sdk_<version>.zip` from the same release into a `packages`
directory in your gamemode project. Keep the archive intact and install it as a
Composer artifact:

```bash
mkdir -p packages
composer config repositories.ompphp artifact packages
composer config platform.php 8.2.0
composer require ompphp/sdk:^0.1
```

Run Composer on your development machine, then deploy the gamemode together with its `vendor/` directory. By default, ompphp loads `gamemode.php` from the server directory. Set `OMPPHP_ENTRY` to use a different entry file.

## Small example

```php
<?php

require __DIR__ . '/vendor/autoload.php';

use Omp\Player;
use Omp\Event\Events;
use Omp\Runtime;
use Omp\Server;

Runtime::assertCompatible();

Server::on(Events::PLAYER_CONNECT, static function (int $playerId): void {
    (new Player($playerId))->sendMessage('Welcome to the server.');
});
```

Common open.mp values are available as grouped constants. For example,
`WeaponID::M4` is `31`, while key flags can be combined with
`Keys::FIRE | Keys::AIM`. The classes live in the `Omp\Constant` namespace.

The complete native API is exposed as static methods grouped under `Omp\Api`:

```php
use Omp\Api\Dialog;
use Omp\Api\Player;
use Omp\Constant\DialogStyle;

Player::setHealth($playerId, 100.0);
Dialog::show($playerId, 1, DialogStyle::MSG_BOX, 'Hello', 'Welcome!', 'OK', '');
```

See the `examples` directory for commands, dialogs, and a minimal gamemode.

## Developer notes

The open.mp API snapshot lives in `third_party/openmp-capi`. Curated gamemode
values that are not part of CAPI metadata live in
`tools/codegen/data/gamemode_constants.json`. Run `task generate` after updating
either source, and commit the regenerated Go, C, and PHP bindings.

Use `task check` for the full Go and PHP test suite. `task e2e` builds the component, downloads a pinned x86-64 artifact from the open.mp build workflow, and starts a test server with the fixture gamemode. For a native-architecture diagnostic build, use `task component:host`.
To test a different open.mp workflow run, set both `OPENMP_WORKFLOW_RUN` and
`OPENMP_ARTIFACT_SHA256`; the download is rejected unless its checksum matches.

Goro and its dependencies are pinned in `go.mod` and checked into `vendor/`. Goro currently assumes a Unix filesystem in several places, so the vendored copy contains a small portability layer used by the Windows build.
The patch and upgrade procedure are documented in `docs/vendor-goro.md`.

PHP execution is serialized through one long-lived Goro runtime. This is intentional: open.mp callbacks are synchronous, and PHP state must survive between events.

Set `OMPPHP_SLOW_CALLBACK_MS` to a positive number to log PHP callbacks that
take at least that many milliseconds. The runtime also tracks dispatch count,
failure count, total callback time, and the longest callback internally.

The SDK targets PHP 8.2 because that is the language version provided by the pinned Goro revision. Gamemode projects should set Composer's `config.platform.php` to `8.2.0` and only require extensions that Goro provides.
