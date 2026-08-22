<?php

declare(strict_types=1);

namespace Omp\Internal;

/** @phpstan-impure */
function native_call(string $name, mixed ...$arguments): mixed
{
    return \ompphp_native_call($name, ...array_values($arguments));
}

function runtime_version(): string
{
    return \ompphp_runtime_version();
}

function api_version(): int
{
    return \ompphp_api_version();
}

function runtime_context(): string
{
    return \ompphp_runtime_context();
}

/** @return array{string, string, int, int, int, int, int}|null */
function component_get(string $uid): ?array { return \ompphp_component_get($uid); }
function component_supports(string $uid, string $interfaceUid, int $abiVersion, int $structSize): bool { return \ompphp_component_supports($uid, $interfaceUid, $abiVersion, $structSize); }
function component_watch(string $uid): int { return \ompphp_component_watch($uid); }
function component_unwatch(int $id): bool { return \ompphp_component_unwatch($id); }
/** @return list<array{string, string, list<array{string, int, bool, bool, mixed}>, int, bool, bool}> */
function component_callables(string $uid): array { return \ompphp_component_callables($uid); }
/** @param list<mixed> $arguments */
function component_invoke(string $uid, string $name, array $arguments): mixed
{
    $result = \ompphp_component_invoke($uid, $name, $arguments);
    if (!is_array($result) || count($result) !== 4 || !is_bool($result[0]) || !is_int($result[1]) || !is_string($result[2])) {
        throw new \UnexpectedValueException('ompphp returned an invalid callable invocation envelope.');
    }
    if (!$result[0]) throw new \RuntimeException($result[2], $result[1]);
    return $result[3];
}
function dispatch_component_invalidated(int $id, string $uid): void { \Omp\Component\Components::dispatchInvalidated($id, $uid); }
function network_subscribe(int $direction, int $id, int $priority, bool $all): int { return \ompphp_network_subscribe($direction, $id, $priority, $all); }
function network_unsubscribe(int $id): bool { return \ompphp_network_unsubscribe($id); }
function network_send(bool $rpc, int $playerId, int $messageId, string $data, int $bitLength, int $channel, bool $dispatchEvents): int { return \ompphp_network_send($rpc, $playerId, $messageId, $data, $bitLength, $channel, $dispatchEvents); }
/** @return list<int> */
function network_types(): array { return \ompphp_network_types(); }
/** @return array{int, int, int, int, int} */
function network_stats(): array { return \ompphp_network_stats(); }

/** @return array{bool, string, int, int} */
function dispatch_network(int $subscriptionId, int $playerId, int $messageId, string $data, int $bitLength, int $readOffsetBits): array
{
    return \Omp\Network\Network::dispatch($subscriptionId, $playerId, $messageId, $data, $bitLength, $readOffsetBits);
}

function async_run(string $class, mixed $payload): int
{
    try {
        return \ompphp_async_run($class, $payload);
    } catch (\Throwable $error) {
        if (str_contains($error->getMessage(), 'scheduler queue is full')) {
            throw new \Omp\Concurrency\SchedulerOverloadedException('The async task queue is full.', previous: $error);
        }
        throw $error;
    }
}

function async_native(string $name, mixed $payload): int
{
    try {
        return \ompphp_async_native($name, $payload);
    } catch (\Throwable $error) {
        if (str_contains($error->getMessage(), 'scheduler queue is full')) {
            throw new \Omp\Concurrency\SchedulerOverloadedException('The async task queue is full.', previous: $error);
        }
        throw $error;
    }
}

function future_cancel(int $id): bool
{
    return \ompphp_future_cancel($id);
}

function future_timeout(int $id, int $milliseconds): void
{
    \ompphp_future_timeout($id, $milliseconds);
}

/** @return array{int, int} */
function actor_spawn(string $class, mixed $payload): array
{
    try {
        return \ompphp_actor_spawn($class, $payload);
    } catch (\Throwable $error) {
        if (str_contains($error->getMessage(), 'scheduler queue is full')) {
            throw new \Omp\Concurrency\SchedulerOverloadedException('The async task queue is full.', previous: $error);
        }
        throw $error;
    }
}

function actor_call(int $id, string $method, mixed $payload): int
{
    try {
        return \ompphp_actor_call($id, $method, $payload);
    } catch (\Throwable $error) {
        if (str_contains($error->getMessage(), 'actor mailbox is full')) {
            throw new \Omp\Concurrency\ActorMailboxFullException('The actor mailbox is full.', previous: $error);
        }
        if (str_contains($error->getMessage(), 'scheduler queue is full')) {
            throw new \Omp\Concurrency\SchedulerOverloadedException('The async task queue is full.', previous: $error);
        }
        throw $error;
    }
}

function actor_stop(int $id): int
{
    return \ompphp_actor_stop($id);
}

/** @return array{int, list<array{int, int}>} */
function actor_pool_spawn(string $class, int $count, mixed $payload): array
{
    try {
        return \ompphp_actor_pool_spawn($class, $count, $payload);
    } catch (\Throwable $error) {
        if (str_contains($error->getMessage(), 'scheduler queue is full')) {
            throw new \Omp\Concurrency\SchedulerOverloadedException('The async task queue is full.', previous: $error);
        }
        throw $error;
    }
}

/** @return list<int> */
function actor_pool_stop(int $id): array
{
    return \ompphp_actor_pool_stop($id);
}

function timer_start(int $milliseconds, bool $repeat): int
{
    return \ompphp_timer_start($milliseconds, $repeat);
}

function timer_cancel(int $id): bool
{
    return \ompphp_timer_cancel($id);
}

/** @return array<string, int> */
function concurrency_stats(): array
{
    return \ompphp_concurrency_stats();
}

/** @param array{class?: string, message?: string, file?: string, line?: int, trace?: string, worker?: int, task?: int}|null $error */
function complete_future(int $id, mixed $value, ?array $error, bool $cancelled): void
{
    \Omp\Concurrency\Future::complete($id, $value, $error, $cancelled);
}

function fire_timer(int $id): void
{
    \Omp\Timer::fire($id);
}

/** @return array<string, list<string>> */
function load_composer_prefixes(string $path): array
{
    $data = require $path;
    if (!is_array($data)) {
        throw new \UnexpectedValueException("Composer PSR-4 metadata at {$path} must return an array.");
    }
    $prefixes = [];
    foreach ($data as $prefix => $directories) {
        if (!is_string($prefix) || !is_array($directories)) {
            throw new \UnexpectedValueException("Composer PSR-4 metadata at {$path} is invalid.");
        }
        $prefixes[$prefix] = [];
        foreach ($directories as $directory) {
            if (!is_string($directory)) {
                throw new \UnexpectedValueException("Composer PSR-4 metadata at {$path} is invalid.");
            }
            $prefixes[$prefix][] = $directory;
        }
    }
    return $prefixes;
}

/** @return array<string, string> */
function load_composer_class_map(string $path): array
{
    $data = require $path;
    if (!is_array($data)) {
        throw new \UnexpectedValueException("Composer class map at {$path} must return an array.");
    }
    $classMap = [];
    foreach ($data as $class => $file) {
        if (!is_string($class) || !is_string($file)) {
            throw new \UnexpectedValueException("Composer class map at {$path} is invalid.");
        }
        $classMap[$class] = $file;
    }
    return $classMap;
}

function install_composer_compatibility_loader(): void
{
    if (!function_exists('ompphp_api_version')) {
        return;
    }
    $composerDirectory = null;
    foreach (get_included_files() as $file) {
        if (str_ends_with(str_replace('\\', '/', $file), '/vendor/composer/autoload_real.php')) {
            $composerDirectory = dirname($file);
            break;
        }
    }
    if ($composerDirectory === null) {
        return;
    }
    $prefixes = load_composer_prefixes($composerDirectory . '/autoload_psr4.php');
    $classMap = load_composer_class_map($composerDirectory . '/autoload_classmap.php');
    spl_autoload_register(static function (string $class) use ($prefixes, $classMap): void {
        if (isset($classMap[$class])) {
            require $classMap[$class];
            return;
        }
        foreach ($prefixes as $prefix => $directories) {
            if (!str_starts_with($class, $prefix)) {
                continue;
            }
            $relative = str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
            foreach ($directories as $directory) {
                $path = $directory . '/' . $relative;
                if (is_file($path)) {
                    require $path;
                    return;
                }
            }
        }
    }, true, true);
}

install_composer_compatibility_loader();

final class HandlerRegistry
{
    /** @var array<string, list<callable>> */
    public static array $handlers = [];
}

/** @phpstan-impure */
function register_handler(string $event, callable $handler): void
{
    HandlerRegistry::$handlers[$event][] = $handler;
}

function format_handler_failure(string $event, \Throwable $error): string
{
    return sprintf("PHP handler for %s failed:\n%s", $event, (string) $error);
}

if (!function_exists(__NAMESPACE__ . '\\dispatch')) {
    /** @param list<mixed> $arguments */
    function dispatch(string $event, array $arguments = [], bool $defaultResult = true): ?bool
    {
        if (!isset(HandlerRegistry::$handlers[$event])) {
            return null;
        }
        $result = $defaultResult;
        foreach (HandlerRegistry::$handlers[$event] as $handler) {
            try {
                $value = $handler(...$arguments);
            } catch (\Throwable $error) {
                error_log(format_handler_failure($event, $error));
                continue;
            }
            if (is_bool($value)) {
                $result = $value;
            }
        }
        return $result;
    }
}
