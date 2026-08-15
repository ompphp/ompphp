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
    public static bool $invalidScore = false;
    public static bool $invalidKeys = false;
}

/** @param list<mixed> $arguments */
function expectLastNativeCall(string $name, array $arguments): void
{
    $lastCall = array_key_last(NativeStub::$calls);
    if ($lastCall === null) {
        throw new RuntimeException('No native call was recorded.');
    }
    expect(NativeStub::$calls[$lastCall] === [$name, $arguments]);
}

function ompphp_native_call(string $name, mixed ...$arguments): mixed
{
    NativeStub::$calls[] = [$name, array_values($arguments)];
    if ($name === 'Player_GetScore' && NativeStub::$invalidScore) {
        return 'invalid';
    }
    if ($name === 'Player_GetKeys' && NativeStub::$invalidKeys) {
        return [132, 'invalid', 1];
    }
    return match ($name) {
        'Player_GetHealth', 'Player_GetArmor' => 75.5,
        'Player_GetScore', 'Player_GetMoney' => 123,
        'Player_GetPos' => [1.5, 2.5, 3.5],
        'Player_GetVelocity' => [0.1, 0.2, 0.3],
        'Player_GetRotationQuat' => [1.0, 0.0, 0.0, 0.0],
        'Player_GetKeys' => [132, -1, 1],
        'Actor_GetPos' => [6.0, 7.0, 8.0],
        'Vehicle_GetPos' => [4.5, 5.5, 6.5],
        'Vehicle_GetVelocity' => [0.4, 0.5, 0.6],
        'Vehicle_GetRotationQuat' => [0.5, 0.5, 0.5, 0.5],
        'Vehicle_GetDamageStatus' => [1, 2, 3, 4],
        default => true,
    };
}
function ompphp_runtime_version(): string { return '0.1.0-test'; }
function ompphp_api_version(): int { return 2; }

require dirname(__DIR__) . '/vendor/autoload.php';

use Omp\Server;
use Omp\Player;
use Omp\Runtime;
use Omp\Value\Vector3;
use Omp\Actor;
use Omp\Api\Player as PlayerAPI;
use Omp\Constant\Keys;
use Omp\Vehicle;

$calls = 0;
Server::on('PlayerConnect', static function (int $id) use (&$calls): bool {
    $calls += $id;
    return false;
});

expect(Server::dispatch('PlayerConnect', 4) === false);
expect($calls === 4);
expect(Server::dispatch('Unknown') === true);

Server::on('NoOpinion', static function (): void {});
expect(\Omp\Internal\dispatch('NoOpinion', [], false) === false);

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
NativeStub::$invalidScore = true;
try {
    $player->score();
    throw new RuntimeException('Invalid scalar data was accepted.');
} catch (UnexpectedValueException $error) {
    expect($error->getMessage() === 'Player_GetScore returned invalid int data.');
} finally {
    NativeStub::$invalidScore = false;
}
expect($player->giveMoney(500));
expect($player->money() === 123);
expect($player->kick());
expect($player->setPosition(new Vector3(10.0, 20.0, 30.0)));
$position = $player->position();
expect($position->x === 1.5 && $position->y === 2.5 && $position->z === 3.5);
$velocity = $player->velocity();
expect($velocity->x === 0.1 && $velocity->y === 0.2 && $velocity->z === 0.3);
expect($player->setVelocity(new Vector3(1.0, 2.0, 3.0)));
expectLastNativeCall('Player_SetVelocity', [7, 1.0, 2.0, 3.0]);
$rotation = $player->rotation();
expect($rotation->w === 1.0 && $rotation->x === 0.0 && $rotation->y === 0.0 && $rotation->z === 0.0);
$keyState = $player->keyState();
expect($keyState->keys === 132 && $keyState->upDown === -1 && $keyState->leftRight === 1);
expect($keyState->pressed(Keys::FIRE | Keys::AIM));
NativeStub::$invalidKeys = true;
try {
    $player->keyState();
    throw new RuntimeException('Invalid tuple element was accepted.');
} catch (UnexpectedValueException $error) {
    expect($error->getMessage() === 'Player_GetKeys returned invalid int output at index 1.');
} finally {
    NativeStub::$invalidKeys = false;
}
expect(NativeStub::$calls[0] === ['Player_SetHealth', [7, 90.0]]);
expect(Runtime::apiVersion() === 2);
expect(Runtime::version() === '0.1.0-test');
Runtime::assertCompatible();

$futureValue = 0;
$futureFinally = false;
$future = \Omp\Concurrency\Future::fromHandle(9001);
$chained = $future->then(static fn (int $value): int => $value * 2)
    ->finally(static function () use (&$futureFinally): void { $futureFinally = true; });
$chained->then(static function (mixed $value) use (&$futureValue): void { $futureValue = $value; });
\Omp\Concurrency\Future::complete(9001, 21, null, false);
expect($future->isFulfilled());
expect($chained->isFulfilled());
expect($futureValue === 42 && $futureFinally);
$lateValue = 0;
$future->then(static function (int $value) use (&$lateValue): void { $lateValue = $value; });
expect($lateValue === 21);

$remoteCaught = false;
$failed = \Omp\Concurrency\Future::fromHandle(9002);
$failed->catch(static function (\Throwable $error) use (&$remoteCaught): void {
    $remoteCaught = $error instanceof \Omp\Concurrency\RemoteTaskException
        && str_contains($error->getMessage(), 'worker failed');
});
\Omp\Concurrency\Future::complete(9002, null, ['class' => 'RuntimeException', 'message' => 'worker failed'], false);
expect($failed->isRejected() && $remoteCaught);

$inner = \Omp\Concurrency\Future::fromHandle(9004);
$outer = \Omp\Concurrency\Future::fromHandle(9003);
$flattened = $outer->then(static fn (): \Omp\Concurrency\Future => $inner);
\Omp\Concurrency\Future::complete(9003, null, null, false);
expect($flattened->isPending());
\Omp\Concurrency\Future::complete(9004, 'done', null, false);
expect($flattened->isFulfilled());

$chainRoot = \Omp\Concurrency\Future::fromHandle(9005);
$chainTail = $chainRoot;
for ($index = 0; $index < 100_000; $index++) {
    $chainTail = $chainTail->then(static fn (int $value): int => $value + 1);
}
\Omp\Concurrency\Future::complete(9005, 0, null, false);
expect($chainTail->isFulfilled());

expect(\Omp\Internal\native_call('Named_Arguments', player: 7) === true);
expectLastNativeCall('Named_Arguments', [7]);

expect(PlayerAPI::setHealth(8, 88.0));
expectLastNativeCall('Player_SetHealth', [8, 88.0]);
expect(PlayerAPI::getKeys(7) === [132, -1, 1]);

$actor = new Actor(2);
expect($actor->setHealth(80.0));
expect($actor->setVirtualWorld(3));
expect($actor->setPosition(new Vector3(1.0, 2.0, 3.0)));
$actorPosition = $actor->position();
expect($actorPosition->x === 6.0 && $actorPosition->y === 7.0 && $actorPosition->z === 8.0);
expect($actor->destroy());

$vehicle = new Vehicle(9);
expect($vehicle->setHealth(900.0));
expect($vehicle->setVirtualWorld(4));
expect($vehicle->setPosition(new Vector3(4.0, 5.0, 6.0)));
$vehiclePosition = $vehicle->position();
expect($vehiclePosition->x === 4.5 && $vehiclePosition->y === 5.5 && $vehiclePosition->z === 6.5);
$vehicleVelocity = $vehicle->velocity();
expect($vehicleVelocity->x === 0.4 && $vehicleVelocity->y === 0.5 && $vehicleVelocity->z === 0.6);
expect($vehicle->setVelocity(new Vector3(7.0, 8.0, 9.0)));
expectLastNativeCall('Vehicle_SetVelocity', [9, 7.0, 8.0, 9.0]);
$vehicleRotation = $vehicle->rotation();
expect($vehicleRotation->w === 0.5 && $vehicleRotation->x === 0.5 && $vehicleRotation->y === 0.5 && $vehicleRotation->z === 0.5);
$damage = $vehicle->damageStatus();
expect($damage->panels === 1 && $damage->doors === 2 && $damage->lights === 3 && $damage->tires === 4);
expect($vehicle->updateDamageStatus(new \Omp\Value\VehicleDamageStatus(5, 6, 7, 8)));
expectLastNativeCall('Vehicle_UpdateDamageStatus', [9, 5, 6, 7, 8]);
expect($vehicle->repair());
expect($vehicle->destroy());

try {
    Vector3::fromNative([1.0, 2.0], 'Invalid_Test');
    throw new RuntimeException('Invalid vector data was accepted.');
} catch (UnexpectedValueException $error) {
    expect($error->getMessage() === 'Invalid_Test returned invalid vector data.');
}

try {
    Vector3::fromNative([1.0, 'invalid', 3.0], 'Invalid_Test');
    throw new RuntimeException('Invalid vector element was accepted.');
} catch (UnexpectedValueException $error) {
    expect($error->getMessage() === 'Invalid_Test returned invalid vector data.');
}

echo "SDK tests passed\n";
