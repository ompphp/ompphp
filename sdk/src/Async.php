<?php

declare(strict_types=1);

namespace Omp;

use Omp\Concurrency\Future;

final class Async
{
    public static function run(string $task, mixed $payload = null): Future
    {
        return Future::fromHandle(\Omp\Internal\async_run($task, $payload));
    }

    public static function native(string $provider, mixed $payload = null): Future
    {
        return Future::fromHandle(\Omp\Internal\async_native($provider, $payload));
    }

    /** @return array<string, int> */
    public static function stats(): array
    {
        return \Omp\Internal\concurrency_stats();
    }
}
