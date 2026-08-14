<?php

declare(strict_types=1);

namespace Omp\Value;

final readonly class Vector3
{
    public function __construct(
        public float $x,
        public float $y,
        public float $z,
    ) {}
}
