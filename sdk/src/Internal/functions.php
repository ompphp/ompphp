<?php

declare(strict_types=1);

namespace Omp\Internal;

/** @phpstan-impure */
function native_call(string $name, mixed ...$arguments): mixed
{
    return \ompphp_native_call($name, ...$arguments);
}

function runtime_version(): string
{
    return \ompphp_runtime_version();
}

function api_version(): int
{
    return \ompphp_api_version();
}

function install_composer_compatibility_loader(): void
{
    if (!function_exists('ompphp_api_version')) {
        return;
    }
    $composerDirectory = null;
    foreach (get_included_files() as $file) {
        if (str_ends_with(str_replace('\\', '/', $file), '/vendor/composer/autoload_real.php')) {
            $composerDirectory = dirname($file);
            break;
        }
    }
    if ($composerDirectory === null) {
        return;
    }
    $prefixes = require $composerDirectory . '/autoload_psr4.php';
    $classMap = require $composerDirectory . '/autoload_classmap.php';
    spl_autoload_register(static function (string $class) use ($prefixes, $classMap): void {
        if (isset($classMap[$class])) {
            require $classMap[$class];
            return;
        }
        foreach ($prefixes as $prefix => $directories) {
            if (!str_starts_with($class, $prefix)) {
                continue;
            }
            $relative = str_replace('\\', '/', substr($class, strlen($prefix))) . '.php';
            foreach ($directories as $directory) {
                $path = $directory . '/' . $relative;
                if (is_file($path)) {
                    require $path;
                    return;
                }
            }
        }
    }, true, true);
}

install_composer_compatibility_loader();

final class HandlerRegistry
{
    /** @var array<string, list<callable>> */
    public static array $handlers = [];
}

/** @phpstan-impure */
function register_handler(string $event, callable $handler): void
{
    HandlerRegistry::$handlers[$event][] = $handler;
}

function format_handler_failure(string $event, \Throwable $error): string
{
    return sprintf("PHP handler for %s failed:\n%s", $event, (string) $error);
}

if (!function_exists(__NAMESPACE__ . '\\dispatch')) {
    /** @param list<mixed> $arguments */
    function dispatch(string $event, array $arguments = []): ?bool
    {
        if (!isset(HandlerRegistry::$handlers[$event])) {
            return null;
        }
        $result = true;
        foreach (HandlerRegistry::$handlers[$event] as $handler) {
            try {
                $value = $handler(...$arguments);
            } catch (\Throwable $error) {
                error_log(format_handler_failure($event, $error));
                continue;
            }
            if (is_bool($value)) {
                $result = $value;
            }
        }
        return $result;
    }
}
