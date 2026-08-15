<?php

declare(strict_types=1);

namespace Example\Concurrency;

final class Square
{
    public function __invoke(mixed $value): int
    {
        $number = (int) $value;
        return $number * $number;
    }
}
