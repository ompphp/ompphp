<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Dialog;
use Omp\Api\Player;
use Omp\Constant\DialogStyle;
use Omp\Event\Handlers;
use Omp\Runtime;

const WELCOME_DIALOG = 1;

Runtime::assertCompatible();

Handlers::playerConnect(static function (int $playerId): void {
    Dialog::show(
        $playerId,
        WELCOME_DIALOG,
        DialogStyle::MSG_BOX,
        'Welcome',
        'This gamemode is running on ompphp.',
        'Continue',
        '',
    );
});

Handlers::dialogResponse(static function (
    int $playerId,
    int $dialogId,
    int $response,
    int $listItem,
    string $inputText,
): void {
    if ($dialogId === WELCOME_DIALOG && $response === 1) {
        Player::sendClientMessage($playerId, -1, 'Thanks for trying ompphp.');
    }
});
