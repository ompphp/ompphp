<?php

declare(strict_types=1);

namespace Omp\Value;

final readonly class Quaternion
{
    public function __construct(
        public float $w,
        public float $x,
        public float $y,
        public float $z,
    ) {}

    public static function fromNative(mixed $value, string $operation): self
    {
        if (!is_array($value) || !array_is_list($value) || count($value) !== 4) {
            throw new \UnexpectedValueException(sprintf('%s returned invalid quaternion data.', $operation));
        }

        return new self((float) $value[0], (float) $value[1], (float) $value[2], (float) $value[3]);
    }
}
