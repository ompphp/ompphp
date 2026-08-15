# ompphp

`ompphp` runs open.mp gamemodes written in PHP. It embeds [Goro](https://github.com/KarpelesLab/goro) in the server component, so production servers don't need a separate PHP installation.

The PHP SDK exposes the open.mp API and events through bindings generated from the official C API metadata.
Async tasks and actors run in isolated PHP workers, then deliver their results to the main gamemode runtime.

## Requirements

- A 64-bit Linux or Windows open.mp server
- The open.mp `$CAPI` component
- Composer on the development machine

`ompphp` doesn't support older 32-bit server releases. The SDK targets PHP 8.2, the version provided by the pinned Goro revision.

## Install the component

Download the component archive for your platform from the GitHub release. Copy `ompphp.so` or `ompphp.dll` from the archive's `components` directory into the server's `components` directory.

To build the component from source:

```sh
git clone --recurse-submodules https://github.com/ompphp/ompphp.git
cd ompphp
task component          # Linux x64
task component:windows  # Windows x64, cross-compiled from Linux
```

Build output is written to `build/`.

## Install the SDK

Download `ompphp-sdk_<version>.zip` from the same release and keep the archive intact in your gamemode project's `packages` directory. Install it as a Composer artifact:

```sh
mkdir -p packages
composer config repositories.ompphp artifact packages
composer config platform.php 8.2.0
composer require ompphp/sdk:^0.1
```

Run Composer on the development machine. Deploy the gamemode with its `vendor/` directory.

Only require PHP extensions that Goro provides.

## Usage

Create `gamemode.php` in the server directory:

```php
<?php

require __DIR__ . '/vendor/autoload.php';

use Omp\Event\Handlers;
use Omp\Player;
use Omp\Runtime;

Runtime::assertCompatible();

Handlers::playerConnect(static function (int $playerId): void {
    (new Player($playerId))->sendMessage('Welcome to the server.');
});
```

Native API methods are grouped under `Omp\Api`:

```php
use Omp\Api\Dialog;
use Omp\Api\Player;
use Omp\Constant\DialogStyle;

Player::setHealth($playerId, 100.0);
Dialog::show($playerId, 1, DialogStyle::MSG_BOX, 'Hello', 'Welcome!', 'OK', '');
```

Common open.mp values are grouped under `Omp\Constant`. For example, `WeaponID::M4` is `31`, and key flags can be combined with `Keys::FIRE | Keys::AIM`.

See [`examples`](examples) for commands, dialogs, and complete gamemodes.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `OMPPHP_ENTRY` | `gamemode.php` | Gamemode entry file, relative to the server directory |
| `OMPPHP_SLOW_CALLBACK_MS` | Disabled | Log callbacks that take at least this many milliseconds; set it to a positive number |
| `OMPPHP_WORKERS` | `4` | Number of isolated PHP workers |
| `OMPPHP_TASK_QUEUE` | `256` | Pending tasks allowed per worker |
| `OMPPHP_COMPLETION_QUEUE` | `512` | Results waiting for the main runtime |
| `OMPPHP_ACTOR_MAILBOX` | `64` | Pending calls allowed per actor |
| `OMPPHP_TRANSFER_MAX_DEPTH` | `32` | Maximum nested payload depth |
| `OMPPHP_TRANSFER_MAX_BYTES` | `1048576` | Maximum payload size in bytes |
| `OMPPHP_WORKER_BOOTSTRAP` | `vendor/autoload.php` beside the entry file | Composer autoloader used by workers |

Gamemode state lives in one long-running, serialized PHP runtime. Async tasks and actors run in isolated workers; they return data to the main runtime and cannot call open.mp directly.

The runtime tracks callback dispatches, failures, total execution time, and the longest callback internally.

## Development

Run the Go and PHP checks:

```sh
task check
```

Build and test the component in an official open.mp Linux server:

```sh
task e2e
```

The end-to-end test downloads a pinned x86-64 artifact from the open.mp build workflow. To test another workflow run, set both `OPENMP_WORKFLOW_RUN` and `OPENMP_ARTIFACT_SHA256`; the download fails if the checksum doesn't match.

Use `task component:host` for a native-architecture diagnostic build.

Go integrations can register cancellable background operations with `async.Register`. PHP starts them through `Async::native()` and receives the result through a `Future` on the main runtime.

### Generated bindings

The open.mp API snapshot is in `third_party/openmp-capi`. Curated gamemode values not included in the CAPI metadata are in `tools/codegen/data/gamemode_constants.json`.

Regenerate the Go, C, and PHP bindings after changing either source:

```sh
task generate
```

Commit the generated files with the source changes.

### Vendored Goro

Goro and its dependencies are pinned in `go.mod` and checked into `vendor/`. The vendored copy includes a small portability layer because Goro assumes a Unix filesystem in several places.

See [Updating vendored Goro](docs/vendor-goro.md) for the patch and upgrade process.
