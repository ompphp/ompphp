<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Player;
use Omp\Event\Events;
use Omp\Runtime;
use Omp\Server;

Runtime::assertCompatible();

Server::on(Events::PLAYER_CONNECT, static function (int $playerId): void {
    (new Player($playerId))->sendMessage('Welcome to the server.');
});
