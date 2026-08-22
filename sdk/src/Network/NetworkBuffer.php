<?php
declare(strict_types=1);
namespace Omp\Network;
final class NetworkBuffer
{
    public function __construct(public string $data, public int $bitLength, public readonly int $readOffsetBits) { $this->validate(); }
    public function replace(string $data, ?int $bitLength = null): void { $this->data = $data; $this->bitLength = $bitLength ?? strlen($data) * 8; $this->validate(); }
    private function validate(): void
    {
        if ($this->bitLength < 0 || $this->bitLength > strlen($this->data) * 8) throw new \InvalidArgumentException('Network bit length exceeds the payload capacity.');
    }
}
