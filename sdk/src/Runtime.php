<?php

declare(strict_types=1);

namespace Omp;

final class Runtime
{
    public const REQUIRED_API_VERSION = 3;

    public static function version(): string
    {
        return \Omp\Internal\runtime_version();
    }

    public static function apiVersion(): int
    {
        return \Omp\Internal\api_version();
    }

    public static function isMain(): bool
    {
        return \Omp\Internal\runtime_context() === 'main';
    }

    public static function isWorker(): bool
    {
        return \Omp\Internal\runtime_context() === 'worker';
    }

    public static function isActor(): bool
    {
        return \Omp\Internal\runtime_context() === 'actor';
    }

    public static function assertCompatible(): void
    {
        if (!function_exists('ompphp_api_version')) {
            throw new \RuntimeException('ompphp is not loaded. Run this gamemode through the ompphp component.');
        }
        if (self::apiVersion() !== self::REQUIRED_API_VERSION) {
            throw new \RuntimeException(sprintf(
                'The SDK requires ompphp native API %d; API %d is loaded.',
                self::REQUIRED_API_VERSION,
                self::apiVersion(),
            ));
        }
    }
}
