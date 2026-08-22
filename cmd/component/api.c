#include "../../third_party/omp-capi/lib/open.mp-capi/include/ompcapi.h"

struct OMPAPI_t ompphp_api;

int ompphp_player_id(void* player) {
    return player && ompphp_api.Player.GetID ? ompphp_api.Player.GetID(player) : -1;
}
