<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Omp\Api\All;
use Omp\Api\Core;
use Omp\Api\Player;
use Omp\Api\PlayerClass;
use Omp\Api\TextDraw;
use Omp\Api\Vehicle;
use Omp\Constant\CameraMoveType;
use Omp\Constant\Keys;
use Omp\Constant\PlayerMarkersMode;
use Omp\Constant\PlayerState;
use Omp\Constant\TextDrawFont;
use Omp\Constant\WeaponID;
use Omp\Event\Handlers;
use Omp\Runtime;

final class GrandLarceny
{
    private const LOS_SANTOS = 0;
    private const SAN_FIERRO = 1;
    private const LAS_VENTURAS = 2;
    private const CITY_SWITCH_DELAY = 0.5;

    /** @var array<int, array{city:int, citySet:bool, selected:bool, lastSwitch:float}> */
    private array $players = [];

    /** @param array<string, list<array{float, float, float, float}>> $spawns */
    public function __construct(
        private readonly array $spawns,
        private int $selectionHelp = -1,
        /** @var array<int, int> */
        private array $cityText = [],
    ) {}

    public function start(): void
    {
        Runtime::assertCompatible();
        $this->configureServer();
        $this->createTextDraws();
        $this->createClasses();
        $this->loadStaticVehicles(__DIR__ . '/scriptfiles/vehicles');

        Handlers::playerConnect(function (int $player): void {
            $this->players[$player] = $this->newPlayerData();
            Player::showGameText($player, '~w~Grand Larceny', 3000, 4);
            Player::sendClientMessage($player, -1, 'Welcome to {88AA88}G{FFFFFF}rand {88AA88}L{FFFFFF}arceny');
        });

        Handlers::playerDisconnect(function (int $player, int $reason): void {
            unset($this->players[$player]);
        });

        Handlers::playerSpawn(function (int $player): void {
            $this->spawnPlayer($player);
        });

        Handlers::playerDeath(function (int $player, int $killer, int $reason): void {
            $this->ensurePlayer($player);
            $this->players[$player]['selected'] = false;
            $cash = Player::getMoney($player);
            if ($killer >= 0 && $cash > 0) {
                Player::giveMoney($killer, $cash);
            }
            Player::resetMoney($player);
        });

        Handlers::playerRequestClass(function (int $player, int $classId): bool {
            if (Player::isNpc($player)) {
                return true;
            }
            $this->ensurePlayer($player);
            if ($this->players[$player]['selected']) {
                $this->setupCharacterSelection($player);
                return true;
            }
            if (Player::getState($player) !== PlayerState::SPECTATING) {
                Player::toggleSpectating($player, true);
                TextDraw::showForPlayer($player, $this->selectionHelp);
                $this->players[$player]['citySet'] = false;
            }
            return false;
        });

        Handlers::playerUpdate(function (int $player): bool {
            if (Player::isNpc($player)) {
                return true;
            }
            $this->ensurePlayer($player);
            if (!$this->players[$player]['selected'] && Player::getState($player) === PlayerState::SPECTATING) {
                $this->handleCitySelection($player);
                return true;
            }
            if (Player::getWeapon($player) === WeaponID::MINIGUN) {
                Player::kick($player);
                return false;
            }
            return true;
        });
    }

    private function configureServer(): void
    {
        Core::gameModeSetText('Grand Larceny');
        Core::showPlayerMarkers(PlayerMarkersMode::GLOBAL);
        Core::showNameTags(true);
        Core::setNameTagsDrawDistance(40.0);
        All::enableStuntBonus(false);
        Core::disableEntryExitMarkers();
        Core::setWeather(2);
        Core::setWorldTime(11);
    }

    private function createTextDraws(): void
    {
        $this->selectionHelp = TextDraw::create(
            10.0,
            415.0,
            ' Press ~b~~k~~GO_LEFT~ ~w~or ~b~~k~~GO_RIGHT~ ~w~to switch cities.~n~ Press ~r~~k~~PED_FIREWEAPON~ ~w~to select.',
        );
        TextDraw::setUseBox($this->selectionHelp, true);
        TextDraw::setBoxColor($this->selectionHelp, 0x222222BB);
        TextDraw::setLetterSize($this->selectionHelp, 0.3, 1.0);
        TextDraw::setTextSize($this->selectionHelp, 400.0, 40.0);
        TextDraw::setFont($this->selectionHelp, TextDrawFont::FONT_BANK_GOTHIC);
        TextDraw::setShadow($this->selectionHelp, 0);
        TextDraw::setOutline($this->selectionHelp, 1);
        TextDraw::setBackgroundColor($this->selectionHelp, 0x000000FF);
        TextDraw::setColor($this->selectionHelp, 0xFFFFFFFF);

        foreach ([
            self::LOS_SANTOS => 'Los Santos',
            self::SAN_FIERRO => 'San Fierro',
            self::LAS_VENTURAS => 'Las Venturas',
        ] as $city => $name) {
            $textDraw = TextDraw::create(10.0, 380.0, $name);
            TextDraw::setUseBox($textDraw, false);
            TextDraw::setLetterSize($textDraw, 1.25, 3.0);
            TextDraw::setFont($textDraw, TextDrawFont::FONT_BECKETT_REGULAR);
            TextDraw::setShadow($textDraw, 0);
            TextDraw::setOutline($textDraw, 1);
            TextDraw::setColor($textDraw, 0xEEEEEEFF);
            $this->cityText[$city] = $textDraw;
        }
    }

    private function createClasses(): void
    {
        $skins = [
            298, 299, 300, 301, 302, 303, 304, 305, 280, 281, 282, 283, 284, 285, 286, 287, 288, 289,
            265, 266, 267, 268, 269, 270, 1, 2, 3, 4, 5, 6, 8, 42, 65, 86, 119, 149, 208, 273, 289, 47,
            48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 68, 69, 70, 71, 72, 73, 75, 76, 78, 79, 80, 81,
            82, 83, 84, 85, 87, 88, 89, 91, 92, 93, 95, 96, 97, 98, 99,
        ];
        foreach ($skins as $skin) {
            PlayerClass::add(255, $skin, 1759.0189, -1898.1260, 13.5622, 266.4503, 0, 0, 0, 0, 0, 0);
        }
    }

    private function spawnPlayer(int $player): void
    {
        if (Player::isNpc($player)) {
            return;
        }
        $this->ensurePlayer($player);
        if (!$this->players[$player]['citySet']) {
            $this->players[$player]['city'] = self::LOS_SANTOS;
            $this->players[$player]['citySet'] = true;
        }
        $key = match ($this->players[$player]['city']) {
            self::SAN_FIERRO => 'san_fierro',
            self::LAS_VENTURAS => 'las_venturas',
            default => 'los_santos',
        };
        $locations = $this->spawns[$key];
        [$x, $y, $z, $angle] = $locations[array_rand($locations)];
        Player::setInterior($player, 0);
        Player::toggleClock($player, false);
        Player::resetMoney($player);
        Player::giveMoney($player, 30000);
        Player::setPos($player, $x, $y, $z);
        Player::setFacingAngle($player, $angle);
        Player::giveWeapon($player, WeaponID::COLT45, 100);
    }

    private function handleCitySelection(int $player): void
    {
        if (!$this->players[$player]['citySet']) {
            $this->switchCity($player, 1);
            return;
        }
        if (microtime(true) - $this->players[$player]['lastSwitch'] < self::CITY_SWITCH_DELAY) {
            return;
        }
        $keyState = Player::getKeys($player);
        $keys = $keyState[0];
        $leftRight = $keyState[2];
        if (($keys & Keys::FIRE) !== 0) {
            $this->players[$player]['selected'] = true;
            $this->hideCityText($player);
            TextDraw::hideForPlayer($player, $this->selectionHelp);
            Player::toggleSpectating($player, false);
            return;
        }
        if ($leftRight !== 0) {
            $this->switchCity($player, $leftRight > 0 ? 1 : -1);
        }
    }

    private function switchCity(int $player, int $direction): void
    {
        if (!$this->players[$player]['citySet']) {
            $city = self::LOS_SANTOS;
            $this->players[$player]['citySet'] = true;
        } else {
            $city = ($this->players[$player]['city'] + $direction + 3) % 3;
        }
        $this->players[$player]['city'] = $city;
        $this->players[$player]['lastSwitch'] = microtime(true);
        Player::playGameSound($player, $direction > 0 ? 1052 : 1053, 0.0, 0.0, 0.0);
        $this->setupSelectedCity($player);
    }

    private function setupSelectedCity(int $player): void
    {
        Player::setInterior($player, 0);
        match ($this->players[$player]['city']) {
            self::SAN_FIERRO => $this->setCamera($player, -1300.8754, 68.0546, 129.4823, -1817.9412, 769.3878, 132.6589),
            self::LAS_VENTURAS => $this->setCamera($player, 1310.6155, 1675.9182, 110.7390, 2285.2944, 1919.3756, 68.2275),
            default => $this->setCamera($player, 1630.6136, -2286.0298, 110.0, 1887.6034, -1682.1442, 47.6167),
        };
        $this->hideCityText($player);
        TextDraw::showForPlayer($player, $this->cityText[$this->players[$player]['city']]);
    }

    private function setupCharacterSelection(int $player): void
    {
        match ($this->players[$player]['city']) {
            self::SAN_FIERRO => $this->setCharacterCamera($player, 3, -2673.8381, 1399.7424, 918.3516, 181.0, -2673.2776, 1394.3859, 918.3516),
            self::LAS_VENTURAS => $this->setCharacterCamera($player, 3, 349.0453, 193.2271, 1014.1797, 286.25, 352.9164, 194.5702, 1014.1875),
            default => $this->setCharacterCamera($player, 11, 508.7362, -87.4335, 998.9609, 0.0, 508.7362, -83.4335, 998.9609),
        };
    }

    private function setCamera(int $player, float $x, float $y, float $z, float $lookX, float $lookY, float $lookZ): bool
    {
        Player::setCameraPos($player, $x, $y, $z);
        return Player::setCameraLookAt($player, $lookX, $lookY, $lookZ, CameraMoveType::MOVE);
    }

    private function setCharacterCamera(int $player, int $interior, float $x, float $y, float $z, float $angle, float $cameraX, float $cameraY, float $cameraZ): bool
    {
        Player::setInterior($player, $interior);
        Player::setPos($player, $x, $y, $z);
        Player::setFacingAngle($player, $angle);
        Player::setCameraPos($player, $cameraX, $cameraY, $cameraZ);
        return Player::setCameraLookAt($player, $x, $y, $z, CameraMoveType::MOVE);
    }

    private function hideCityText(int $player): void
    {
        foreach ($this->cityText as $textDraw) {
            TextDraw::hideForPlayer($player, $textDraw);
        }
    }

    private function ensurePlayer(int $player): void
    {
        $this->players[$player] ??= $this->newPlayerData();
    }

    /** @return array{city:int, citySet:bool, selected:bool, lastSwitch:float} */
    private function newPlayerData(): array
    {
        return ['city' => self::LOS_SANTOS, 'citySet' => false, 'selected' => false, 'lastSwitch' => microtime(true)];
    }

    private function loadStaticVehicles(string $directory): void
    {
        if (!is_dir($directory)) {
            Core::log('[GrandLarceny] Optional scriptfiles/vehicles directory not found; continuing without static vehicles.');
            return;
        }
        $count = 0;
        foreach (glob($directory . '/*.txt') ?: [] as $path) {
            foreach (file($path, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES) ?: [] as $line) {
                $parts = array_map('trim', explode(',', $line));
                if (count($parts) < 7) {
                    continue;
                }
                Vehicle::addStaticEx(
                    (int) $parts[0], (float) $parts[1], (float) $parts[2], (float) $parts[3],
                    (float) $parts[4], (int) $parts[5], (int) $parts[6], 1800, false,
                );
                $count++;
            }
        }
        Core::log(sprintf('[GrandLarceny] Loaded %d static vehicles.', $count));
    }
}

(new GrandLarceny(require __DIR__ . '/spawns.php'))->start();
