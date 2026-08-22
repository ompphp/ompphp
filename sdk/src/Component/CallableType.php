<?php
declare(strict_types=1);
namespace Omp\Component;
enum CallableType: int
{
    case Null = 0;
    case Bool = 1;
    case Int32 = 2;
    case UInt32 = 3;
    case Int64 = 4;
    case UInt64 = 5;
    case Float = 6;
    case Double = 7;
    case String = 8;
    case Bytes = 9;
    case Entity = 10;
}
