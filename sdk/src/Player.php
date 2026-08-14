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
        return \Omp\Internal\player_set_health($this->id, $health);
    }

    public function health(): float
    {
        return \Omp\Internal\player_get_health($this->id);
    }

    public function setArmor(float $armor): bool
    {
        return \Omp\Internal\player_set_armor($this->id, $armor);
    }

    public function armor(): float
    {
        return \Omp\Internal\player_get_armor($this->id);
    }

    public function setScore(int $score): bool
    {
        return \Omp\Internal\player_set_score($this->id, $score);
    }

    public function score(): int
    {
        return \Omp\Internal\player_get_score($this->id);
    }

    public function giveMoney(int $amount): bool
    {
        return \Omp\Internal\player_give_money($this->id, $amount);
    }

    public function money(): int
    {
        return \Omp\Internal\player_get_money($this->id);
    }

    public function kick(): bool
    {
        return \Omp\Internal\player_kick($this->id);
    }

    public function setPosition(Vector3 $position): bool
    {
        return \Omp\Internal\player_set_pos(
            $this->id,
            $position->x,
            $position->y,
            $position->z,
        );
    }

    public function position(): Vector3
    {
        return Vector3::fromNative(\Omp\Internal\player_get_pos($this->id), 'Player_GetPos');
    }

    public function velocity(): Vector3
    {
        return Vector3::fromNative(\Omp\Internal\player_get_velocity($this->id), 'Player_GetVelocity');
    }

    public function setVelocity(Vector3 $velocity): bool
    {
        return \Omp\Internal\player_set_velocity(
            $this->id,
            $velocity->x,
            $velocity->y,
            $velocity->z,
        );
    }

    public function rotation(): Quaternion
    {
        return Quaternion::fromNative(\Omp\Internal\player_get_rotation_quat($this->id), 'Player_GetRotationQuat');
    }

    public function keyState(): KeyState
    {
        return KeyState::fromNative(\Omp\Internal\player_get_keys($this->id));
    }

    public function sendMessage(string $message, int $colour = -1): bool
    {
        return \Omp\Internal\player_send_client_message($this->id, $colour, $message);
    }
}
