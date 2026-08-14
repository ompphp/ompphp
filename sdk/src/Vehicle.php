<?php

declare(strict_types=1);

namespace Omp;

use Omp\Value\Vector3;

final readonly class Vehicle
{
    public function __construct(public int $id) {}

    public function setHealth(float $health): bool
    {
        return (bool) \Omp\Internal\vehicle_set_health($this->id, $health);
    }

    public function health(): float
    {
        return (float) \Omp\Internal\vehicle_get_health($this->id);
    }

    public function setVirtualWorld(int $world): bool
    {
        return (bool) \Omp\Internal\vehicle_set_virtual_world($this->id, $world);
    }

    public function virtualWorld(): int
    {
        return (int) \Omp\Internal\vehicle_get_virtual_world($this->id);
    }

    public function setPosition(Vector3 $position): bool
    {
        return (bool) \Omp\Internal\vehicle_set_pos($this->id, $position->x, $position->y, $position->z);
    }

    public function repair(): bool
    {
        return (bool) \Omp\Internal\vehicle_repair($this->id);
    }

    public function destroy(): bool
    {
        return (bool) \Omp\Internal\vehicle_destroy($this->id);
    }
}
