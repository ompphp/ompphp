<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Player;
use Omp\Constant\WeaponID;
use Omp\Event\Handlers;
use Omp\Runtime;

Runtime::assertCompatible();

Handlers::playerCommandText(static function (int $playerId, string $command): bool {
    return match ($command) {
        '/heal' => Player::setHealth($playerId, 100.0),
        '/m4' => Player::giveWeapon($playerId, WeaponID::M4, 200),
        default => false,
    };
});
