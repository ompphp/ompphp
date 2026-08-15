<?php

declare(strict_types=1);

namespace Omp;

final class Timer
{
    /** @var array<int, array{callable, bool}> */
    private static array $callbacks = [];

    private function __construct(private readonly int $handle) {}

    public static function after(int $milliseconds, callable $callback): self
    {
        return self::create($milliseconds, $callback, false);
    }

    public static function every(int $milliseconds, callable $callback): self
    {
        return self::create($milliseconds, $callback, true);
    }

    public function cancel(): bool
    {
        unset(self::$callbacks[$this->handle]);
        return \Omp\Internal\timer_cancel($this->handle);
    }

    /** @internal */
    public static function fire(int $handle): void
    {
        $entry = self::$callbacks[$handle] ?? null;
        if ($entry === null) {
            return;
        }
        [$callback, $repeat] = $entry;
        if (!$repeat) {
            unset(self::$callbacks[$handle]);
        }
        $callback();
    }

    private static function create(int $milliseconds, callable $callback, bool $repeat): self
    {
        $handle = \Omp\Internal\timer_start($milliseconds, $repeat);
        self::$callbacks[$handle] = [$callback, $repeat];
        return new self($handle);
    }
}
