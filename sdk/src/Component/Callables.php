<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class Callables
{
    public function __construct(private Component $component) {}
    /** @return list<ComponentCallable> */
    public function all(): array
    {
        $result = [];
        foreach (\Omp\Internal\component_callables($this->component->uid) as $value) $result[] = new ComponentCallable($this->component, self::descriptor($value));
        return $result;
    }
    public function find(string $name): ?ComponentCallable
    {
        foreach ($this->all() as $callable) if ($callable->descriptor->name === $name) return $callable;
        return null;
    }
    public function has(string $name): bool { return $this->find($name) !== null; }
    public function require(string $name): ComponentCallable
    {
        return $this->find($name) ?? throw new CallableUnavailableException("Required callable {$this->component->name}::$name is unavailable.");
    }
    /** @param array{string, string, list<array{string, int, bool, bool, mixed}>, int, bool, bool} $value */
    private static function descriptor(array $value): CallableDescriptor
    {
        $parameters = [];
        foreach ($value[2] as $parameter) $parameters[] = new CallableParameter($parameter[0], CallableType::from($parameter[1]), $parameter[2], $parameter[3], $parameter[4]);
        return new CallableDescriptor($value[0], $value[1], $parameters, CallableType::from($value[3]), $value[4], $value[5]);
    }
}
