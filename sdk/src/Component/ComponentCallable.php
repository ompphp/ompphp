<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class ComponentCallable
{
    public function __construct(public Component $component, public CallableDescriptor $descriptor) {}
    public function invoke(mixed ...$arguments): mixed
    {
        $normalized = [];
        foreach ($arguments as $argument) $normalized[] = self::normalize($argument);
        try {
            $result = \Omp\Internal\component_invoke($this->component->uid, $this->descriptor->name, $normalized);
        } catch (\Throwable $error) {
            throw new CallableInvocationException("Invocation of {$this->component->name}::{$this->descriptor->name} failed: {$error->getMessage()}", code: $error->getCode(), previous: $error);
        }
        return $this->result($result);
    }
    /** @param array<string, mixed> $arguments */
    public function invokeNamed(array $arguments): mixed
    {
        $ordered = [];
        foreach ($this->descriptor->parameters as $parameter) {
            if (array_key_exists($parameter->name, $arguments)) {
                $ordered[] = $arguments[$parameter->name];
                unset($arguments[$parameter->name]);
            } elseif ($parameter->hasDefault) {
                $ordered[] = $parameter->default;
            } elseif (!$parameter->optional) {
                throw new \InvalidArgumentException("Missing required callable argument: {$parameter->name}");
            } else {
                break;
            }
        }
        if ($arguments !== []) throw new \InvalidArgumentException('Unknown callable argument: ' . (string) array_key_first($arguments));
        return $this->invoke(...$ordered);
    }
    private static function normalize(mixed $value): mixed
    {
        if ($value instanceof UnsignedInteger) return $value->value;
        if ($value instanceof EntityValue) return [$value->type, $value->id->value];
        return $value;
    }
    private function result(mixed $value): mixed
    {
        if ($this->descriptor->returnType === CallableType::UInt64) {
            if (!is_int($value) && !is_string($value)) throw new \UnexpectedValueException('Callable returned invalid uint64 data.');
            return new UnsignedInteger($value);
        }
        if ($this->descriptor->returnType === CallableType::Entity) {
            if (!is_array($value) || count($value) !== 2 || !is_int($value[0]) || (!is_int($value[1]) && !is_string($value[1]))) throw new \UnexpectedValueException('Callable returned invalid entity data.');
            return new EntityValue($value[0], $value[1]);
        }
        return $value;
    }
}
