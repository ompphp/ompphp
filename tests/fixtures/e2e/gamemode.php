<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Core;
use Omp\Event\Handlers;
use Omp\Player;
use Omp\Runtime;
use Omp\Value\KeyState;
use Omp\Value\Quaternion;
use Omp\Value\Vector3;
use Omp\Value\VehicleDamageStatus;

Runtime::assertCompatible();

$player = new Player(7);
$position = new Vector3(1.0, 2.0, 3.0);
$keyState = KeyState::fromNative([132, -1, 1]);
$rotation = Quaternion::fromNative([1.0, 0.0, 0.0, 0.0], 'E2E_Rotation');
$damage = VehicleDamageStatus::fromNative([1, 2, 3, 4]);
if (
    $player->id !== 7
    || $position->z !== 3.0
    || !$keyState->pressed(132)
    || $rotation->w !== 1.0
    || $damage->tires !== 4
) {
    throw new RuntimeException('Readonly SDK value objects returned unexpected data.');
}

Core::log('OMPPHP_E2E_READY');
Core::log('OMPPHP_E2E_SDK');

Handlers::tick(static function (): void {
    if (empty($GLOBALS['ompphp_e2e_failure'])) {
        $GLOBALS['ompphp_e2e_failure'] = true;
        throw new RuntimeException('OMPPHP_E2E_EXPECTED_FAILURE');
    }
});

Handlers::tick(static function (): void {
    if (empty($GLOBALS['ompphp_e2e_tick'])) {
        $GLOBALS['ompphp_e2e_tick'] = true;
        Core::log('OMPPHP_E2E_TICK');
    }
});
