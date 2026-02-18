#pragma once

#include <iostream>
#include <vector>
#include <memory>
#include <thread>
#include <deque>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <concepts>
#include <tuple>
// #include <sys/stat.h>
// #include <fstream>
#include <random>
// #include <iterator>
// #include <algorithm>
#include <assert.h>
// #include <fcntl.h>
// #include <stdio.h>
#include <map>
// #include <queue>
// #include <sstream>
#include <future>
// #include <functional>
// #include <condition_variable>
// #include <mutex>
// #include <chrono>
// #include <atomic>
#include <set>
// #include <unordered_set>
#include <unordered_map>
// #include <stack>
#include <optional>
#include <mutex>
// #include <chrono>
// #include <math.h>
#include <type_traits>
// #include <ctime>
#include <functional>
// #include <fcntl.h>
#include <string>
#include <iomanip>
#include <cstdio>
#include "Stopwatch.h"
#include "concurrent.h"

#define DISALLOW_COPY(ClassName)           \
    ClassName(const ClassName &) = delete; \
    ClassName & operator=(const ClassName &) = delete

#define DISALLOW_MOVE(ClassName)      \
    ClassName(ClassName &&) = delete; \
    ClassName & operator=(ClassName &&) = delete

#define DISALLOW_COPY_AND_MOVE(ClassName) \
    DISALLOW_COPY(ClassName);             \
    DISALLOW_MOVE(ClassName)

#define log(info) log__(__func__, __LINE__, info)
#define panic(info) panic__(__func__, __LINE__, info)
#define print(...) print__(__VA_ARGS__)

void log__();

template<typename T>
void log__(std::string func, int line, T &info) {
    std::cout << func << ", line " << line << ": " << info << std::endl;
}

template<typename T>
void panic__(std::string func, int line, T &info) {
    std::cout << "Panic: " << func << ", line " << line << ": " << info << std::endl;
    exit(-1);
}

inline void print__() { std::cout << std::endl; };

template<typename T, typename... Types>
void print__(const T& firstArg, const Types&... args) {
    std::cout << firstArg << std::ends;
    print__(args...);
}

template<typename T>
void printVec(const std::vector<T>& v) {
    for (auto &item : v)
        std::cout << item << " ";

    std::cout << std::endl;
}

template<typename T>
void printDeq(const std::deque<T>& dq) {
    for (auto &item : dq)
        std::cout << item << " ";

    std::cout << std::endl;
}

template<typename T1, typename T2>
void printPair(const std::pair<T1, T2>& p) {
    std::cout << "(" << p.first << ", " << p.second << ") ";
}

class TreeNode {
public:
    int val;
    TreeNode *left;
    TreeNode *right;
    TreeNode() : val(0), left(nullptr), right(nullptr) {}
    TreeNode(int x) : val(x), left(nullptr), right(nullptr) {}
    TreeNode(int x, TreeNode *left, TreeNode *right) : val(x), left(left), right(right) {}
 };

class Node {
public:
    int val;
    std::vector<Node*> children;

    Node() {}

    Node(int _val) {
        val = _val;
    }

    Node(int _val, std::vector<Node*> _children) {
        val = _val;
        children = _children;
    }
};

class ListNode {
public:
    int val;
    ListNode *next;
    ListNode() : val(0), next(nullptr) {}
    ListNode(int x) : val(x), next(nullptr) {}
    ListNode(int x, ListNode *next) : val(x), next(next) {}
};


// range: [begin, end]
class RandNum {
public:
    RandNum(uint64_t begin, uint64_t end) : di(begin, end) {}

    void setRange(uint64_t begin, uint64_t end) {
        di.param(std::uniform_int_distribution<uint64_t>::param_type{begin, end});
    }

    int64_t getNum() { return static_cast<int64_t>(di(dre)); }
private:
    std::default_random_engine dre;
    std::uniform_int_distribution<uint64_t> di;
};

class RandString {
public:
    explicit RandString() : rand_num_(int(' '), int('~')) {}

    std::string getRandString(size_t str_len) {
        std::string ret;
        ret.resize(str_len);
        for (size_t i = 0; i < str_len; i++) {
            ret[i] = rand_num_.getNum();
        }
        return ret;
    }

private:
    RandNum rand_num_;
};

struct Noisy {
    Noisy() { std::cout << "constructed at " << this << '\n'; }
    Noisy(const Noisy&) { std::cout << "copy-constructed\n"; }
    Noisy(Noisy&&) { std::cout << "move-constructed\n"; }
    ~Noisy() { std::cout << "destructed at " << this << '\n'; }

    void func() noexcept;
};

using String = std::string;
using StringView = std::string_view;
