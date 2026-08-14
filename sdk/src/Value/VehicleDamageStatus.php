<?php

declare(strict_types=1);

namespace Omp\Value;

final readonly class VehicleDamageStatus
{
    public function __construct(
        public int $panels,
        public int $doors,
        public int $lights,
        public int $tires,
    ) {}

    public static function fromNative(mixed $value): self
    {
        if (
            !is_array($value)
            || !array_is_list($value)
            || count($value) !== 4
            || !is_int($value[0])
            || !is_int($value[1])
            || !is_int($value[2])
            || !is_int($value[3])
        ) {
            throw new \UnexpectedValueException('Vehicle_GetDamageStatus returned invalid damage data.');
        }

        return new self($value[0], $value[1], $value[2], $value[3]);
    }
}
