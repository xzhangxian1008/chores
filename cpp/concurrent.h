#pragma once

#include <memory>
#include <thread>
#include <tuple>
#include <iostream>
#include <condition_variable>
#include "Stopwatch.h"

namespace {

std::mutex mu;
std::condition_variable cv;
size_t thread_ready_num = 0;
std::atomic_bool start(false);
static constexpr size_t thd_num = 300;
constexpr size_t action_num = 9999999;
constexpr size_t data_num_per_thd = 1024;
// constexpr size_t padding_data = 0; // 10 * 1024 * 1024 = 10Mbi
constexpr size_t padding_data = 10 * 1024 * 1024; // 10 * 1024 * 1024 = 10Mbi
// char* sharing_mem;

struct SharingMem {
    char padding[thd_num * (data_num_per_thd + padding_data)];
};

SharingMem sharing_mem;

size_t convertToIndex(int index, size_t i) {
    size_t actual_i = i % data_num_per_thd;
    return actual_i + (index * (data_num_per_thd + padding_data));
}

template <typename Func>
void thread_run(Func && func, int thd_idx) {
    std::unique_lock<std::mutex> lock(mu);
    thread_ready_num++;
    cv.wait(lock, [&](){
        if (start)
            return true;
        return false;
    });
    lock.unlock();

    std::apply(std::forward<Func>(func), std::make_tuple(thd_idx));

    lock.lock();
    thread_ready_num--;
    if (thread_ready_num == 0) {
        cv.notify_all();
    }
    lock.unlock();
}

template <typename Func>
void start_tasks(Func &&func) {
    for (size_t i = 0; i < thd_num; ++i) {
        // Always place the thread id at the first parameter
        std::thread thd(thread_run<decltype(func)>, std::forward<Func>(func), i);
        thd.detach();
    }

    std::unique_lock<std::mutex> ul(mu, std::defer_lock);
    auto begin = std::chrono::steady_clock::now();
    while (true) {
        std::this_thread::sleep_for(std::chrono::milliseconds(5));
        ul.lock();
        if (thread_ready_num == thd_num) {
            start = true;
            ul.unlock();
            break;
        }
        ul.unlock();
    }

    cv.notify_all();
    Stopwatch sw;
    begin = std::chrono::steady_clock::now();
    ul.lock();
    cv.wait(ul);
    sw.stop();
    std::cout << "Elapsed time: " << sw.elapsedMilliseconds() << "ms" << std::endl;
}
}
