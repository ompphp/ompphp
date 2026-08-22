<?php
declare(strict_types=1);
namespace Omp\Component;
final readonly class EntityValue
{
    public UnsignedInteger $id;
    public function __construct(public int $type, string|int|UnsignedInteger $id)
    {
        if ($type <= 0) throw new \InvalidArgumentException('Entity type must be positive.');
        $this->id = $id instanceof UnsignedInteger ? $id : new UnsignedInteger($id);
    }
}
