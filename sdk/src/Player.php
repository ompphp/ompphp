<?php

declare(strict_types=1);

namespace Omp;

use Omp\Value\KeyState;
use Omp\Value\Quaternion;
use Omp\Value\Vector3;

final readonly class Player
{
    public function __construct(public int $id) {}

    public function setHealth(float $health): bool
    {
        return (bool) \Omp\Internal\native_call('Player_SetHealth', $this->id, $health);
    }

    public function health(): float
    {
        return (float) \Omp\Internal\native_call('Player_GetHealth', $this->id);
    }

    public function setArmor(float $armor): bool
    {
        return (bool) \Omp\Internal\native_call('Player_SetArmor', $this->id, $armor);
    }

    public function armor(): float
    {
        return (float) \Omp\Internal\native_call('Player_GetArmor', $this->id);
    }

    public function setScore(int $score): bool
    {
        return (bool) \Omp\Internal\native_call('Player_SetScore', $this->id, $score);
    }

    public function score(): int
    {
        return (int) \Omp\Internal\native_call('Player_GetScore', $this->id);
    }

    public function giveMoney(int $amount): bool
    {
        return (bool) \Omp\Internal\native_call('Player_GiveMoney', $this->id, $amount);
    }

    public function money(): int
    {
        return (int) \Omp\Internal\native_call('Player_GetMoney', $this->id);
    }

    public function kick(): bool
    {
        return (bool) \Omp\Internal\native_call('Player_Kick', $this->id);
    }

    public function setPosition(Vector3 $position): bool
    {
        return (bool) \Omp\Internal\native_call(
            'Player_SetPos',
            $this->id,
            $position->x,
            $position->y,
            $position->z,
        );
    }

    public function position(): Vector3
    {
        return Vector3::fromNative(\Omp\Internal\native_call('Player_GetPos', $this->id), 'Player_GetPos');
    }

    public function velocity(): Vector3
    {
        return Vector3::fromNative(\Omp\Internal\native_call('Player_GetVelocity', $this->id), 'Player_GetVelocity');
    }

    public function setVelocity(Vector3 $velocity): bool
    {
        return (bool) \Omp\Internal\native_call(
            'Player_SetVelocity',
            $this->id,
            $velocity->x,
            $velocity->y,
            $velocity->z,
        );
    }

    public function rotation(): Quaternion
    {
        return Quaternion::fromNative(\Omp\Internal\native_call('Player_GetRotationQuat', $this->id), 'Player_GetRotationQuat');
    }

    public function keyState(): KeyState
    {
        return KeyState::fromNative(\Omp\Internal\native_call('Player_GetKeys', $this->id));
    }

    public function sendMessage(string $message, int $colour = -1): bool
    {
        return (bool) \Omp\Internal\native_call('Player_SendClientMessage', $this->id, $colour, $message);
    }
}
