<?php

declare(strict_types=1);

function expect(bool $condition): void
{
    if (!$condition) {
        throw new RuntimeException('SDK test expectation failed.');
    }
}

final class NativeStub
{
    /** @var list<array{string, list<mixed>}> */
    public static array $calls = [];
}

function ompphp_native_call(string $name, mixed ...$arguments): mixed
{
    NativeStub::$calls[] = [$name, $arguments];
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
use Omp\Vehicle;

$calls = 0;
Server::on('PlayerConnect', static function (int $id) use (&$calls): bool {
    $calls += $id;
    return false;
});

expect(Server::dispatch('PlayerConnect', 4) === false);
expect($calls === 4);
expect(Server::dispatch('Unknown') === true);

Server::on('Broken', static function (): void {
    throw new RuntimeException('expected test failure');
});
$afterFailure = 0;
Server::on('Broken', static function () use (&$afterFailure): void {
    $afterFailure++;
});
expect(Server::dispatch('Broken') === true);
expect($afterFailure === 1);

$failure = new RuntimeException('diagnostic test');
$diagnostic = \Omp\Internal\format_handler_failure('Broken', $failure);
expect(str_contains($diagnostic, 'PHP handler for Broken failed:'));
expect(str_contains($diagnostic, 'RuntimeException: diagnostic test'));
expect(str_contains($diagnostic, __FILE__ . ':'));
expect(str_contains($diagnostic, 'Stack trace:'));

$player = new Player(7);
expect($player->setHealth(90.0));
expect($player->health() === 75.5);
expect($player->setArmor(50.0));
expect($player->armor() === 75.5);
expect($player->setScore(10));
expect($player->score() === 123);
expect($player->giveMoney(500));
expect($player->money() === 123);
expect($player->kick());
expect($player->setPosition(new Vector3(10.0, 20.0, 30.0)));
$position = $player->position();
expect($position->x === 1.5 && $position->y === 2.5 && $position->z === 3.5);
expect(NativeStub::$calls[0] === ['Player_SetHealth', [7, 90.0]]);
expect(Runtime::apiVersion() === 1);
expect(Runtime::version() === '0.1.0-test');
Runtime::assertCompatible();

expect(PlayerAPI::setHealth(8, 88.0));
$lastCall = array_key_last(NativeStub::$calls);
expect($lastCall !== null);
expect(NativeStub::$calls[$lastCall] === ['Player_SetHealth', [8, 88.0]]);

$actor = new Actor(2);
expect($actor->setHealth(80.0));
expect($actor->setVirtualWorld(3));
expect($actor->setPosition(new Vector3(1.0, 2.0, 3.0)));
expect($actor->destroy());

$vehicle = new Vehicle(9);
expect($vehicle->setHealth(900.0));
expect($vehicle->setVirtualWorld(4));
expect($vehicle->setPosition(new Vector3(4.0, 5.0, 6.0)));
expect($vehicle->repair());
expect($vehicle->destroy());

echo "SDK tests passed\n";
