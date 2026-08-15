<?php

declare(strict_types=1);

namespace OmpPhp\E2E;

final class DoubleTask
{
    public function __invoke(mixed $value): int
    {
        return (int) $value * 2;
    }
}
