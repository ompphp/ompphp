<?php

declare(strict_types=1);

namespace OmpPhp\E2E;

final class CounterActor
{
    public function __construct(private int $value) {}

    public function add(mixed $value): int
    {
        return $this->value += (int) $value;
    }
}
