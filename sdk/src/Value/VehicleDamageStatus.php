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
        if (!is_array($value) || !array_is_list($value) || count($value) !== 4) {
            throw new \UnexpectedValueException('Vehicle_GetDamageStatus returned invalid damage data.');
        }

        return new self((int) $value[0], (int) $value[1], (int) $value[2], (int) $value[3]);
    }
}
