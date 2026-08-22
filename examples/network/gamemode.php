<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Core;
use Omp\Network\Network;
use Omp\Network\NetworkMessage;
use Omp\Network\NetworkResult;
use Omp\Runtime;

Runtime::assertCompatible();

$subscription = Network::onIncomingRpc(
    id: 24,
    handler: static function (NetworkMessage $message): NetworkResult {
        Core::log(sprintf('RPC 24 from player %d contains %d bits', $message->playerId ?? -1, $message->buffer->bitLength));
        return NetworkResult::CONTINUE;
    },
);

Core::log(sprintf('Detected %d network implementation(s)', count(Network::types())));

// Keep $subscription alive while the handler is wanted. Calling cancel()
// safely unregisters it, including when called from inside a callback.
