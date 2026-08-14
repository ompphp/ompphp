<?php

declare(strict_types=1);

namespace Omp;

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
        $position = \Omp\Internal\native_call('Player_GetPos', $this->id);
        if (!is_array($position) || count($position) !== 3) {
            throw new \UnexpectedValueException('Player_GetPos returned invalid position data.');
        }
        return new Vector3((float) $position[0], (float) $position[1], (float) $position[2]);
    }

    public function sendMessage(string $message, int $colour = -1): bool
    {
        return (bool) \Omp\Internal\native_call('Player_SendClientMessage', $this->id, $colour, $message);
    }
}
