<?php
declare(strict_types=1);
namespace Omp\Component;
final class Components
{
    /** @var array<int, callable(string): void> */
    private static array $watches = [];
    public static function find(string $uid): ?Component
    {
        $value = \Omp\Internal\component_get(self::uid($uid));
        if ($value === null) return null;
        return new Component($value[0], $value[1], new ComponentVersion($value[2], $value[3], $value[4], $value[5]), $value[6]);
    }
    public static function require(string $uid): Component
    {
        return self::find($uid) ?? throw new ComponentUnavailableException("Required open.mp component $uid is unavailable.");
    }
    public static function watch(string $uid, callable $onInvalidated): ComponentWatch
    {
        $uid = self::uid($uid);
        $id = \Omp\Internal\component_watch($uid);
        self::$watches[$id] = $onInvalidated;
        return new ComponentWatch($id);
    }
    public static function cancelWatch(int $id): bool
    {
        if (!isset(self::$watches[$id])) return false;
        unset(self::$watches[$id]);
        return \Omp\Internal\component_unwatch($id);
    }
    public static function dispatchInvalidated(int $id, string $uid): void
    {
        $handler = self::$watches[$id] ?? null;
        unset(self::$watches[$id]);
        if ($handler !== null) $handler($uid);
    }
    private static function uid(string $uid): string
    {
        $uid = strtolower(str_starts_with($uid, '0x') ? substr($uid, 2) : $uid);
        if (!preg_match('/^[0-9a-f]{1,16}$/', $uid)) throw new \InvalidArgumentException("Invalid component UID: $uid");
        return str_pad($uid, 16, '0', STR_PAD_LEFT);
    }
}
