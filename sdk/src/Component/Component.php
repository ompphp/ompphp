<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class Component
{
    public function __construct(public string $uid, public string $name, public ComponentVersion $version, public int $type) {}
    public function supports(string $interfaceUid, int $abiVersion = 1, int $minimumStructSize = 16): bool
    {
        return \Omp\Internal\component_supports($this->uid, $interfaceUid, $abiVersion, $minimumStructSize);
    }
    public function isAvailable(): bool { return Components::find($this->uid) !== null; }
    public function watch(callable $onInvalidated): ComponentWatch { return Components::watch($this->uid, $onInvalidated); }
    public function callables(): Callables { return new Callables($this); }
    /** @param list<mixed> $arguments */
    public function call(string $name, array $arguments = []): mixed { return $this->callables()->require($name)->invoke(...$arguments); }
}
