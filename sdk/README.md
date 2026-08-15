# ompphp SDK

This package contains the PHP API for ompphp gamemodes. It is designed for the
PHP 8.2 language level implemented by the embedded runtime.

Release archives can be installed through Composer's artifact repository:

```bash
mkdir -p packages
# Download ompphp-sdk_<version>.zip into packages/, without extracting it.
composer config repositories.ompphp artifact packages
composer require ompphp/sdk:^0.1
```

Require `vendor/autoload.php` from the gamemode and deploy the gamemode together
with its generated `vendor/` directory.

## API layers

Use `Omp\Player`, `Omp\Vehicle`, and `Omp\Actor` for common operations. These facades use readonly objects for positions, velocities, rotations, key state, and vehicle damage:

```php
use Omp\Player;
use Omp\Value\Vector3;

$player = new Player($playerId);
$position = $player->position();
$player->setVelocity(new Vector3(0.0, 0.0, 0.2));
```

Use the static classes under `Omp\Api` for complete access to the generated open.mp API. Methods with several output values return positional arrays.

Register events through `Omp\Event\Handlers`. Its generated methods let static analysis check each callback's parameters:

```php
use Omp\Event\Handlers;

Handlers::playerConnect(static function (int $playerId): void {
    // ...
});
```

## Concurrency

`Async::run()` executes an invokable, Composer-autoloaded class in an isolated PHP worker. Its payload and result may contain only null, booleans, numbers, strings, and arrays. Future callbacks run later on the main PHP runtime, where open.mp calls are safe:

```php
use Omp\Async;

Async::run(CalculatePath::class, $request)
    ->withTimeout(1000)
    ->then(static function (array $path): void {
        // Apply the result to open.mp here.
    })
    ->catch(static function (Throwable $error): void {
        // Handle worker errors, cancellation, and timeouts.
    });
```

Workers cannot call open.mp. They keep separate PHP state and load the gamemode's `vendor/autoload.php`, not `gamemode.php`.

Use `Omp\Concurrency\Actor` for persistent worker-local state and `Omp\Timer` for callbacks on the main runtime. `Future` deliberately has no `await()` method: blocking the main runtime would also block completion delivery.
