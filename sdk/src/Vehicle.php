<?php

declare(strict_types=1);

namespace Omp;

use Omp\Value\Quaternion;
use Omp\Value\Vector3;
use Omp\Value\VehicleDamageStatus;

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

    public function position(): Vector3
    {
        return Vector3::fromNative(\Omp\Internal\vehicle_get_pos($this->id), 'Vehicle_GetPos');
    }

    public function velocity(): Vector3
    {
        return Vector3::fromNative(\Omp\Internal\vehicle_get_velocity($this->id), 'Vehicle_GetVelocity');
    }

    public function setVelocity(Vector3 $velocity): bool
    {
        return (bool) \Omp\Internal\vehicle_set_velocity(
            $this->id,
            $velocity->x,
            $velocity->y,
            $velocity->z,
        );
    }

    public function rotation(): Quaternion
    {
        return Quaternion::fromNative(\Omp\Internal\vehicle_get_rotation_quat($this->id), 'Vehicle_GetRotationQuat');
    }

    public function damageStatus(): VehicleDamageStatus
    {
        return VehicleDamageStatus::fromNative(\Omp\Internal\vehicle_get_damage_status($this->id));
    }

    public function updateDamageStatus(VehicleDamageStatus $status): bool
    {
        return (bool) \Omp\Internal\vehicle_update_damage_status(
            $this->id,
            $status->panels,
            $status->doors,
            $status->lights,
            $status->tires,
        );
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
