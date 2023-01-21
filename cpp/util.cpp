#include "util.h"
#include <mutex>
#include <utility>

void log__() { cout << endl; }

void Noisy::func() noexcept {
    PRINT("func");
}
