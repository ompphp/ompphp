# ompphp

`ompphp` runs open.mp gamemodes in embedded PHP using [Goro](https://github.com/KarpelesLab/goro). The generated SDK provides open.mp APIs and events, component callables, networking, timers, async tasks, and actors.

## Requirements

- A 64-bit Linux or Windows open.mp server
- Composer on the development machine
- PHP 8.2 compatibility

Release archives include the required extended `$CAPI`. The stock open.mp CAPI and 32-bit servers are unsupported.

## Installation

Copy both release components into the server, replacing its stock `$CAPI`:

```text
components/
├── $CAPI.so       # $CAPI.dll on Windows
└── ompphp.so      # ompphp.dll on Windows
```

Keep `ompphp-sdk_<version>.zip` intact in your gamemode's `packages/` directory, then install it:

```sh
composer config repositories.ompphp artifact packages
composer config platform.php 8.2.0
composer require ompphp/sdk:^0.1
```

Run Composer during development and deploy the resulting `vendor/` directory. Only require PHP extensions provided by Goro.

To build from source:

```sh
git clone --recurse-submodules https://github.com/ompphp/ompphp.git
cd ompphp
task component          # Linux x64
task component:windows  # Windows x64 from Linux
```

## Quick start

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

Generated static APIs live under `Omp\Api`, event helpers under `Omp\Event`, and constants under `Omp\Constant`. See [`examples/`](examples) for complete usage.

## Component callables

Components register scripting functions through the CAPI callable registry. ompphp discovers and invokes them without exposing native pointers:

```php
use Omp\Component\Components;

$component = Components::require('0x1234567890abcdef');
$object = $component->callables()->require('CreateDynamicObject')->invokeNamed([
    'model' => 19379,
    'x' => 100.0,
    'y' => 200.0,
    'z' => 10.0,
]);
```

Component-specific Composer packages can wrap these calls with typed PHP APIs, serving the same role as Pawn include packages. Components that do not register callables are not dynamically invokable.

Calls run synchronously on the main thread. `UnsignedInteger` preserves full `uint64` values and `EntityValue` represents typed entity IDs.

## Network API

```php
use Omp\Network\Network;
use Omp\Network\NetworkMessage;
use Omp\Network\NetworkResult;

$subscription = Network::onIncomingRpc(
    24,
    static fn (NetworkMessage $message): NetworkResult => NetworkResult::CONTINUE,
);

Network::sendRpc(playerId: 7, rpcId: 24, data: "\x01\x02");
$subscription->cancel();
```

Subscriptions are synchronous and main-runtime only. Handlers may replace the buffer or return `DROP`; nested callbacks are rejected. Passing `null` as the player ID broadcasts.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `OMPPHP_ENTRY` | `gamemode.php` | Gamemode entry file |
| `OMPPHP_SLOW_CALLBACK_MS` | disabled | Slow callback logging threshold |
| `OMPPHP_WORKERS` / `OMPPHP_TASK_QUEUE` | `4` / `256` | PHP worker count and queue |
| `OMPPHP_NATIVE_WORKERS` / `OMPPHP_NATIVE_QUEUE` | `8` / `256` | Native async workers and queue |
| `OMPPHP_COMPLETION_QUEUE` | `512` | Pending main-runtime results |
| `OMPPHP_ACTOR_MAILBOX` | `64` | Calls queued per actor |
| `OMPPHP_TRANSFER_MAX_DEPTH` / `OMPPHP_TRANSFER_MAX_BYTES` | `32` / `1048576` | Cross-runtime payload limits |
| `OMPPHP_NETWORK_MAX_BYTES` / `OMPPHP_NETWORK_MAX_SUBSCRIPTIONS` | `1048576` / `1024` | Network bridge limits |
| `OMPPHP_CALLABLE_MAX_OUTPUT_BYTES` | `1048576` | Callable string/byte result buffer |
| `OMPPHP_WORKER_BOOTSTRAP` | `vendor/autoload.php` | Worker Composer bootstrap |

The main gamemode runtime is serialized. Workers and actors are isolated and cannot call open.mp directly.

## Development

```sh
task check     # Go and PHP checks
task e2e       # official open.mp Linux x86-64 integration test
task generate  # regenerate C, Go, and PHP bindings
```

The standalone CAPI and metadata are pinned in `third_party/omp-capi`. Use `task component:host` for a native-architecture diagnostic build.

Goro is pinned and vendored with portability patches; see [Updating vendored Goro](docs/vendor-goro.md).
