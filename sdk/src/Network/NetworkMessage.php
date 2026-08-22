<?php
declare(strict_types=1);
namespace Omp\Network;
final readonly class NetworkMessage
{
    public function __construct(public ?int $playerId, public int $id, public NetworkBuffer $buffer) {}
}
