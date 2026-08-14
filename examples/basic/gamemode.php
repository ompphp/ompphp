<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Player;
use Omp\Event\Handlers;
use Omp\Runtime;

Runtime::assertCompatible();

Handlers::playerConnect(static function (int $playerId): void {
    (new Player($playerId))->sendMessage('Welcome to the server.');
});
