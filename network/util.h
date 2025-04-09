#include <netinet/in.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include <cassert>
#include <iostream>

#pragma once

constexpr uint16_t SERVER_PORT = 5762;

inline void print__() { std::cout << std::endl; };

template<typename T, typename... Types>
void print__(const T& firstArg, const Types&... args) {
    std::cout << firstArg << std::ends;
    print__(args...);
}
