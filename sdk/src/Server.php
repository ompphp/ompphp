<?php

declare(strict_types=1);

namespace Omp;

final class Server
{
    /** @param class-string $event */
    public static function on(string $event, callable $handler): void
    {
        \Omp\Internal\register_handler($event, $handler);
    }

    /** @internal */
    public static function dispatch(string $event, mixed ...$arguments): bool
    {
        return \Omp\Internal\dispatch($event, $arguments) ?? true;
    }
}
