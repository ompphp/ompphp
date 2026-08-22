<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\Core;
use Omp\Event\Handlers;
use Omp\Player;
use Omp\Runtime;
use Omp\Vehicle;
use Omp\Value\KeyState;
use Omp\Value\Quaternion;
use Omp\Value\Vector3;
use Omp\Value\VehicleDamageStatus;
use Omp\Async;
use Omp\Concurrency\Actor as WorkerActor;
use Omp\Timer;
use Omp\Component\Components;
use Omp\Network\Network;
use Omp\Network\NetworkDirection;
use Omp\Network\NetworkResult;
use OmpPhp\E2E\CounterActor;
use OmpPhp\E2E\DoubleTask;

Runtime::assertCompatible();

$player = new Player(7);
$position = new Vector3(1.0, 2.0, 3.0);
$keyState = KeyState::fromNative([132, -1, 1]);
$rotation = Quaternion::fromNative([1.0, 0.0, 0.0, 0.0], 'E2E_Rotation');
$damage = VehicleDamageStatus::fromNative([1, 2, 3, 4]);
$playerVelocityResult = $player->setVelocity(new Vector3(0.0, 0.0, 0.2));
$vehicle = new Vehicle(7);
$vehicleVelocityResult = $vehicle->setVelocity(new Vector3(0.0, 0.0, 0.1));
$vehicleDamageResult = $vehicle->updateDamageStatus($damage);
if (
    $player->id !== 7
    || $position->z !== 3.0
    || !$keyState->pressed(132)
    || $rotation->w !== 1.0
    || $damage->tires !== 4
    || !is_bool($playerVelocityResult)
    || !is_bool($vehicleVelocityResult)
    || !is_bool($vehicleDamageResult)
) {
    throw new RuntimeException('Readonly SDK value objects returned unexpected data.');
}

Core::log('OMPPHP_E2E_READY');
Core::log('OMPPHP_E2E_SDK');

$self = Components::require('0x4f4d505048500001');
$networkSubscription = Network::subscribe(
    NetworkDirection::INCOMING_RPC,
    24,
    static fn (): NetworkResult => NetworkResult::CONTINUE,
);
$componentWatch = $self->watch(static function (): void {});
$networkStats = Network::stats();
if (
    $self->name !== 'ompphp'
    || !$self->isAvailable()
    || Network::types() === []
    || $networkStats['subscriptions'] < 1
    || Network::sendPacket(null, '') < 0
    || !$networkSubscription->cancel()
    || !$componentWatch->cancel()
) {
    throw new RuntimeException('Extended CAPI APIs returned unexpected data.');
}
Core::log('OMPPHP_E2E_EXTENDED_CAPI');

$callableFixture = Components::require('4f4d505043414c4c');
$add = $callableFixture->callables()->require('add');
$maximum = $callableFixture->callables()->require('maximumUnsigned')->invoke();
if ($add->invokeNamed(['left' => 20, 'right' => 22]) !== 42 || (string) $maximum !== '18446744073709551615') {
    throw new RuntimeException('CAPI callable registry returned unexpected data.');
}
Core::log('OMPPHP_E2E_CALLABLES');

Async::run(DoubleTask::class, 21)->then(static function (mixed $value): void {
    if ($value === 42) {
        Core::log('OMPPHP_E2E_ASYNC');
    }
});

$counter = WorkerActor::spawn(CounterActor::class, 10);
$counter->call('add', 1);
$counter->call('add', 2)->then(static function (mixed $value): void {
    if ($value === 13) {
        Core::log('OMPPHP_E2E_ACTOR');
    }
});

Timer::after(10, static function (): void {
    Core::log('OMPPHP_E2E_TIMER');
});

Handlers::tick(static function (): void {
    if (empty($GLOBALS['ompphp_e2e_failure'])) {
        $GLOBALS['ompphp_e2e_failure'] = true;
        throw new RuntimeException('OMPPHP_E2E_EXPECTED_FAILURE');
    }
});

Handlers::tick(static function (): void {
    if (empty($GLOBALS['ompphp_e2e_tick'])) {
        $GLOBALS['ompphp_e2e_tick'] = true;
        Core::log('OMPPHP_E2E_TICK');
    }
});
