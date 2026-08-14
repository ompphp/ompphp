<?php

declare(strict_types=1);

namespace Omp\Value;

final readonly class KeyState
{
    public function __construct(
        public int $keys,
        public int $upDown,
        public int $leftRight,
    ) {}

    public function pressed(int $keys): bool
    {
        return ($this->keys & $keys) === $keys;
    }

    public static function fromNative(mixed $value): self
    {
        if (
            !is_array($value)
            || !array_is_list($value)
            || count($value) !== 3
            || !is_int($value[0])
            || !is_int($value[1])
            || !is_int($value[2])
        ) {
            throw new \UnexpectedValueException('Player_GetKeys returned invalid key state data.');
        }

        return new self($value[0], $value[1], $value[2]);
    }
}
