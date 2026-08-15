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
function ompphp_timer_start(int $milliseconds, bool $repeat): int { return 1; }
function ompphp_timer_cancel(int $id): bool { return true; }
/** @return array<string, int> */
function ompphp_concurrency_stats(): array { return []; }
