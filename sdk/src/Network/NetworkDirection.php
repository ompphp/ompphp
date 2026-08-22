<?php
declare(strict_types=1);
namespace Omp\Network;
enum NetworkDirection: int
{
    case INCOMING_PACKET = 0; case OUTGOING_PACKET = 1; case INCOMING_RPC = 2; case OUTGOING_RPC = 3;
}
