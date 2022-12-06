#include "Stopwatch.h"
#include "util.h"
#include <chrono>
#include <cstdint>
#include <exception>
#include <functional>
#include <locale>
#include <memory>
#include <optional>
#include <re2/re2.h>
#include <re2/stringpiece.h>
#include <string>
#include <string_view>
#include <sys/types.h>
#include <thread>
#include <tuple>
#include <type_traits>
#include <bitset>
#include <unordered_map>
using namespace std;

struct C {
void func(string && str_) {
    PRINT(long(&str_));
}

string str;
};

int main(int argc, char * argv[])
{
    PRINT(std::thread::hardware_concurrency());

    return 0;
}
