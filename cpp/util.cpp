#include "util.h"
#include <mutex>
#include <utility>

void log__() { cout << endl; }

using Ptr = std::unique_lock<int>;
