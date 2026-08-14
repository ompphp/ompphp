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
