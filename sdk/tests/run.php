<?php

declare(strict_types=1);

$nativeCalls = [];
function ompphp_native_call(string $name, mixed ...$arguments): mixed
{
    $GLOBALS['nativeCalls'][] = [$name, $arguments];
    return match ($name) {
        'Player_GetHealth', 'Player_GetArmor' => 75.5,
        'Player_GetScore', 'Player_GetMoney' => 123,
        'Player_GetPos' => [1.5, 2.5, 3.5],
        default => true,
    };
}
function ompphp_runtime_version(): string { return '0.1.0-test'; }
function ompphp_api_version(): int { return 1; }

require dirname(__DIR__) . '/vendor/autoload.php';

use Omp\Server;
use Omp\Player;
use Omp\Runtime;
use Omp\Value\Vector3;
use Omp\Actor;
use Omp\Api\Player as PlayerAPI;
use Omp\Constant\Keys;
use Omp\Constant\WeaponID;
use Omp\Vehicle;

$calls = 0;
Server::on('PlayerConnect', static function (int $id) use (&$calls): bool {
    $calls += $id;
    return false;
});

assert(Server::dispatch('PlayerConnect', 4) === false);
assert($calls === 4);
assert(Server::dispatch('Unknown') === true);

Server::on('Broken', static function (): void {
    throw new RuntimeException('expected test failure');
});
assert(Server::dispatch('Broken') === true);

$player = new Player(7);
assert($player->setHealth(90.0));
assert($player->health() === 75.5);
assert($player->setArmor(50.0));
assert($player->armor() === 75.5);
assert($player->setScore(10));
assert($player->score() === 123);
assert($player->giveMoney(500));
assert($player->money() === 123);
assert($player->kick());
assert($player->setPosition(new Vector3(10.0, 20.0, 30.0)));
$position = $player->position();
assert($position->x === 1.5 && $position->y === 2.5 && $position->z === 3.5);
assert($nativeCalls[0] === ['Player_SetHealth', [7, 90.0]]);
assert(Runtime::apiVersion() === 1);
assert(Runtime::version() === '0.1.0-test');
Runtime::assertCompatible();

assert(WeaponID::M4 === 31);
assert((Keys::FIRE | Keys::AIM) === 132);
assert(PlayerAPI::setHealth(8, 88.0));
assert($nativeCalls[array_key_last($nativeCalls)] === ['Player_SetHealth', [8, 88.0]]);

$actor = new Actor(2);
assert($actor->setHealth(80.0));
assert($actor->setVirtualWorld(3));
assert($actor->setPosition(new Vector3(1.0, 2.0, 3.0)));
assert($actor->destroy());

$vehicle = new Vehicle(9);
assert($vehicle->setHealth(900.0));
assert($vehicle->setVirtualWorld(4));
assert($vehicle->setPosition(new Vector3(4.0, 5.0, 6.0)));
assert($vehicle->repair());
assert($vehicle->destroy());

echo "SDK tests passed\n";
