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
function ompphp_api_version(): int { return 5; }
/** @return array{string, string, int, int, int, int, int}|null */
function ompphp_component_get(string $uid): ?array { return $uid === '4f4d505048500001' ? [$uid, 'ompphp', 1, 2, 3, 0, 0] : null; }
function ompphp_component_supports(string $uid, string $interfaceUid, int $version, int $size): bool { return $interfaceUid === '1234567890abcdef' && $version === 1 && $size === 16; }
function ompphp_component_watch(string $uid): int { return 6001; }
function ompphp_component_unwatch(int $id): bool { return $id === 6001; }
/** @return list<array{string, string, list<array{string, int, bool, bool, mixed}>, int, bool, bool}> */
function ompphp_component_callables(string $uid): array
{
    if ($uid !== '4f4d505048500001') return [];
    return [
        ['add', 'Adds two integers.', [['left', 4, false, false, null], ['right', 4, false, false, null]], 4, false, false],
        ['largeId', '', [], 5, false, false],
        ['entity', '', [], 10, false, false],
        ['greet', '', [['name', 8, false, false, null], ['prefix', 8, true, true, 'Hello']], 8, false, false],
        ['fail', '', [], 0, false, false],
    ];
}
/** @param list<mixed> $arguments */
function ompphp_component_invoke(string $uid, string $name, array $arguments): mixed
{
    if ($name === 'fail') return [false, 7, 'rejected by fixture', null];
    $value = match ($name) {
        'add' => 42,
        'largeId' => '18446744073709551615',
        'entity' => [7, '18446744073709551615'],
        'greet' => 'Hello Michael',
        default => throw new RuntimeException('callable not found'),
    };
    return [true, 0, '', $value];
}
function ompphp_network_subscribe(int $direction, int $id, int $priority, bool $all): int { return 7001; }
function ompphp_network_unsubscribe(int $id): bool { return $id === 7001; }
function ompphp_network_send(bool $rpc, int $playerId, int $messageId, string $data, int $bits, int $channel, bool $dispatch): int { return $playerId < 0 ? 3 : 1; }
/** @return list<int> */
function ompphp_network_types(): array { return [0, 1]; }
/** @return array{int, int, int, int, int} */
function ompphp_network_stats(): array { return [1, 2, 1, 0, 100]; }
/** @return array{int, list<array{int, int}>} */
function ompphp_actor_pool_spawn(string $class, int $count, mixed $payload): array
{
    $handles = [];
    for ($index = 0; $index < $count; $index++) { $handles[] = [10_000 + $index, 20_000 + $index]; }
    return [30_000, $handles];
}
function ompphp_actor_call(int $id, string $method, mixed $payload): int { return 40_000 + $id; }
/** @return list<int> */
function ompphp_actor_pool_stop(int $id): array { return [50_001, 50_002, 50_003, 50_004]; }

require dirname(__DIR__) . '/vendor/autoload.php';

use Omp\Server;
use Omp\Player;
use Omp\Runtime;
use Omp\Value\Vector3;
use Omp\Actor;
use Omp\Api\Player as PlayerAPI;
use Omp\Constant\Keys;
use Omp\Vehicle;
use Omp\Component\Components;
use Omp\Component\EntityValue;
use Omp\Component\UnsignedInteger;

$calls = 0;
$component = Components::require('4f4d505048500001');
expect($component->callables()->has('add'));
expect($component->call('add', [20, 22]) === 42);
expect($component->callables()->require('greet')->invokeNamed(['name' => 'Michael']) === 'Hello Michael');
$largeId = $component->callables()->require('largeId')->invoke();
expect($largeId instanceof UnsignedInteger && (string) $largeId === '18446744073709551615');
$entity = $component->callables()->require('entity')->invoke();
expect($entity instanceof EntityValue && $entity->type === 7 && (string) $entity->id === '18446744073709551615');
try {
    $component->callables()->require('fail')->invoke();
    throw new RuntimeException('Rejected callable succeeded.');
} catch (\Omp\Component\CallableInvocationException $error) {
    expect($error->getCode() === 7 && str_contains($error->getMessage(), 'rejected by fixture'));
}
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
expect(Runtime::apiVersion() === 5);
expect(Runtime::version() === '0.1.0-test');
Runtime::assertCompatible();

$component = \Omp\Component\Components::require('0x4f4d505048500001');
expect($component->name === 'ompphp' && (string) $component->version === '1.2.3');
expect($component->supports('1234567890abcdef'));
expect($component->isAvailable());
$invalidated = '';
$componentWatch = $component->watch(static function (string $uid) use (&$invalidated): void { $invalidated = $uid; });
\Omp\Internal\dispatch_component_invalidated($componentWatch->id, $component->uid);
expect($invalidated === $component->uid);
expect(\Omp\Component\Components::find('1') === null);

$networkCalled = false;
$subscription = \Omp\Network\Network::subscribe(\Omp\Network\NetworkDirection::INCOMING_RPC, 24, static function (\Omp\Network\NetworkMessage $message) use (&$networkCalled): \Omp\Network\NetworkResult {
    $networkCalled = $message->playerId === 7 && $message->id === 24 && $message->buffer->data === "a";
    $message->buffer->replace("b");
    return \Omp\Network\NetworkResult::DROP;
});
$networkResult = \Omp\Internal\dispatch_network($subscription->id, 7, 24, "a", 8, 0);
expect($networkCalled && $networkResult === [true, "b", 8, 0]);
expect(\Omp\Network\Network::sendRpc(7, 24, "x") === 1);
expect(\Omp\Network\Network::sendPacket(null, "x") === 3);
expect(\Omp\Network\Network::broadcastRpc(24, "x") === 3);
expect(\Omp\Network\Network::types() === [0, 1]);
expect(\Omp\Network\Network::stats()['callbacks'] === 2);
expect($subscription->cancel());

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

$pool = \Omp\Concurrency\ActorPool::spawn(stdClass::class, 4);
expect($pool->size() === 4);
expect($pool->actorFor('player-42') === $pool->actorFor('player-42'));
expect(count($pool->ready()) === 4);
expect($pool->call('player-42', 'move')->isPending());
expect(count($pool->stop()) === 4);
try {
    $pool->actor(4);
    throw new RuntimeException('An invalid actor pool shard was accepted.');
} catch (OutOfBoundsException) {}

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
