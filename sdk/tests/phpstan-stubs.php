<?php

declare(strict_types=1);

function ompphp_native_call(string $name, mixed ...$arguments): mixed
{
    return null;
}

function ompphp_runtime_version(): string
{
    return '';
}

function ompphp_api_version(): int
{
    return 0;
}

function ompphp_runtime_context(): string { return 'main'; }
function ompphp_async_run(string $class, mixed $payload): int { return 1; }
function ompphp_async_native(string $name, mixed $payload): int { return 1; }
function ompphp_future_cancel(int $id): bool { return true; }
function ompphp_future_timeout(int $id, int $milliseconds): void {}
/** @return array{int, int} */
function ompphp_actor_spawn(string $class, mixed $payload): array { return [1, 2]; }
function ompphp_actor_call(int $id, string $method, mixed $payload): int { return 1; }
function ompphp_actor_stop(int $id): int { return 1; }
/** @return array{int, list<array{int, int}>} */
function ompphp_actor_pool_spawn(string $class, int $count, mixed $payload): array { return [1, [[2, 3]]]; }
/** @return list<int> */
function ompphp_actor_pool_stop(int $id): array { return [1]; }
function ompphp_timer_start(int $milliseconds, bool $repeat): int { return 1; }
function ompphp_timer_cancel(int $id): bool { return true; }
/** @return array<string, int> */
function ompphp_concurrency_stats(): array { return []; }
/** @return array{string, string, int, int, int, int, int}|null */
function ompphp_component_get(string $uid): ?array { return $uid === '' ? ['', '', 0, 0, 0, 0, 0] : null; }
function ompphp_component_supports(string $uid, string $interfaceUid, int $abiVersion, int $structSize): bool { return false; }
function ompphp_component_watch(string $uid): int { return 1; }
function ompphp_component_unwatch(int $id): bool { return true; }
/** @return list<array{string, string, list<array{string, int, bool, bool, mixed}>, int, bool, bool}> */
function ompphp_component_callables(string $uid): array { return []; }
/** @param list<mixed> $arguments */
function ompphp_component_invoke(string $uid, string $name, array $arguments): mixed { return [true, 0, '', null]; }
function ompphp_network_subscribe(int $direction, int $id, int $priority, bool $all): int { return 1; }
function ompphp_network_unsubscribe(int $id): bool { return true; }
function ompphp_network_send(bool $rpc, int $playerId, int $messageId, string $data, int $bitLength, int $channel, bool $dispatchEvents): int { return 1; }
/** @return list<int> */
function ompphp_network_types(): array { return []; }
/** @return array{int, int, int, int, int} */
function ompphp_network_stats(): array { return [0, 0, 0, 0, 0]; }
