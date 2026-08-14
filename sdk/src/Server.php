<?php

declare(strict_types=1);

namespace Omp;

final class Server
{
    /**
     * Register a handler for an event name from {@see \Omp\Event\Events}.
     *
     * Use the generated methods on {@see \Omp\Event\Handlers} when static
     * analysis of the handler parameters is needed.
     */
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
