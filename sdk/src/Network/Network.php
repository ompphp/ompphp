<?php
declare(strict_types=1);
namespace Omp\Network;
final class Network
{
    /** @var array<int, callable(NetworkMessage): mixed> */
    private static array $handlers = [];
    public static function subscribe(NetworkDirection $direction, int $id, callable $handler, int $priority = 0): NetworkSubscription
    {
        return self::register($direction, $id, $handler, $priority, false);
    }
    public static function subscribeAll(NetworkDirection $direction, callable $handler, int $priority = 0): NetworkSubscription
    {
        return self::register($direction, -1, $handler, $priority, true);
    }
    public static function onIncomingPacket(int $id, callable $handler, int $priority = 0): NetworkSubscription { return self::subscribe(NetworkDirection::INCOMING_PACKET, $id, $handler, $priority); }
    public static function onOutgoingPacket(int $id, callable $handler, int $priority = 0): NetworkSubscription { return self::subscribe(NetworkDirection::OUTGOING_PACKET, $id, $handler, $priority); }
    public static function onIncomingRpc(int $id, callable $handler, int $priority = 0): NetworkSubscription { return self::subscribe(NetworkDirection::INCOMING_RPC, $id, $handler, $priority); }
    public static function onOutgoingRpc(int $id, callable $handler, int $priority = 0): NetworkSubscription { return self::subscribe(NetworkDirection::OUTGOING_RPC, $id, $handler, $priority); }
    private static function register(NetworkDirection $direction, int $id, callable $handler, int $priority, bool $all): NetworkSubscription
    {
        if ($priority < -128 || $priority > 127) throw new \InvalidArgumentException('Network priority must be between -128 and 127.');
        $token = \Omp\Internal\network_subscribe($direction->value, $id, $priority, $all);
        self::$handlers[$token] = $handler;
        return new NetworkSubscription($token);
    }
    public static function sendPacket(?int $playerId, string $data, ?int $bitLength = null, int $channel = 0): int { return self::send(false, $playerId, 0, $data, $bitLength, $channel); }
    public static function sendRpc(?int $playerId, int $rpcId, string $data, ?int $bitLength = null, int $channel = 0): int { return self::send(true, $playerId, $rpcId, $data, $bitLength, $channel); }
    public static function broadcastPacket(string $data, ?int $bitLength = null, int $channel = 0): int { return self::sendPacket(null, $data, $bitLength, $channel); }
    public static function broadcastRpc(int $rpcId, string $data, ?int $bitLength = null, int $channel = 0): int { return self::sendRpc(null, $rpcId, $data, $bitLength, $channel); }
    private static function send(bool $rpc, ?int $playerId, int $id, string $data, ?int $bits, int $channel): int
    {
        if ($bits === null) $bits = strlen($data) * 8;
        if ($id < 0 || $channel < 0) throw new \InvalidArgumentException('Network IDs and channels must not be negative.');
        if ($bits < 0 || $bits > strlen($data) * 8) throw new \InvalidArgumentException('Network bit length exceeds the payload capacity.');
        return \Omp\Internal\network_send($rpc, $playerId ?? -1, $id, $data, $bits, $channel, false);
    }
    /** @return list<int> */
    public static function types(): array { return \Omp\Internal\network_types(); }
    /** @return array{subscriptions: int, callbacks: int, dropped: int, rejected: int, callbackNanoseconds: int} */
    public static function stats(): array
    {
        [$subscriptions, $callbacks, $dropped, $rejected, $callbackNanoseconds] = \Omp\Internal\network_stats();
        return ['subscriptions' => $subscriptions, 'callbacks' => $callbacks, 'dropped' => $dropped, 'rejected' => $rejected, 'callbackNanoseconds' => $callbackNanoseconds];
    }
    public static function cancel(int $id): bool { unset(self::$handlers[$id]); return \Omp\Internal\network_unsubscribe($id); }
    /** @return array{bool, string, int, int} */
    public static function dispatch(int $subscriptionId, int $playerId, int $messageId, string $data, int $bitLength, int $readOffsetBits): array
    {
        $buffer = new NetworkBuffer($data, $bitLength, $readOffsetBits);
        $handler = self::$handlers[$subscriptionId] ?? null;
        $result = $handler ? $handler(new NetworkMessage($playerId < 0 ? null : $playerId, $messageId, $buffer)) : null;
        if ($result !== null && !$result instanceof NetworkResult) throw new \UnexpectedValueException('A network handler must return NetworkResult or null.');
        return [$result === NetworkResult::DROP, $buffer->data, $buffer->bitLength, $buffer->readOffsetBits];
    }
}
