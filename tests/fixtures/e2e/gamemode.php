<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Core;
use Omp\Event\Handlers;
use Omp\Player;
use Omp\Runtime;
use Omp\Value\Vector3;

Runtime::assertCompatible();

$player = new Player(7);
$position = new Vector3(1.0, 2.0, 3.0);
if ($player->id !== 7 || $position->z !== 3.0) {
    throw new RuntimeException('Readonly SDK value objects returned unexpected data.');
}

Core::log('OMPPHP_E2E_READY');
Core::log('OMPPHP_E2E_SDK');

Handlers::tick(static function (): void {
    if (empty($GLOBALS['ompphp_e2e_tick'])) {
        $GLOBALS['ompphp_e2e_tick'] = true;
        Core::log('OMPPHP_E2E_TICK');
    }
});
