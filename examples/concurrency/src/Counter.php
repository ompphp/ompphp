<?php

declare(strict_types=1);

namespace Example\Concurrency;

final class Counter
{
    public function __construct(private int $value) {}

    public function add(mixed $amount): int
    {
        return $this->value += (int) $amount;
    }
}
