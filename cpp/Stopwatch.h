#pragma once

#include <chrono>
#include <assert.h>
#include <mutex>

class Stopwatch {
public:
    /** CLOCK_MONOTONIC works relatively efficient (~15 million calls/sec) and doesn't lead to syscall.
      * Pass CLOCK_MONOTONIC_COARSE, if you need better performance with acceptable cost of several milliseconds of inaccuracy.
      */
    explicit Stopwatch(clockid_t clock_type_ = CLOCK_MONOTONIC)
        : clock_type(clock_type_) {
        start();
    }

    void start() {
        start_ns = nanoseconds();
        last_ns = start_ns;
        is_running = true;
    }

    void stop() {
        stop_ns = nanoseconds();
        is_running = false;
    }

    void reset() {
        start_ns = 0;
        stop_ns = 0;
        last_ns = 0;
        is_running = false;
    }

    void restart() { start(); }
    uint64_t elapsed() const { return is_running ? nanoseconds() - start_ns : stop_ns - start_ns; }
    uint64_t elapsedMilliseconds() const { return elapsed() / 1000000UL; }
    double elapsedSeconds() const { return static_cast<double>(elapsed()) / 1000000000ULL; }

    uint64_t elapsedFromLastTime() {
        const auto now_ns = nanoseconds();
        if (is_running)
        {
            auto rc = now_ns - last_ns;
            last_ns = now_ns;
            return rc;
        }
        else
        {
            return stop_ns - last_ns;
        }
    };

    uint64_t elapsedMillisecondsFromLastTime() { return elapsedFromLastTime() / 1000000UL; }
    uint64_t elapsedSecondsFromLastTime() { return elapsedFromLastTime() / 1000000UL; }

private:
    uint64_t start_ns = 0;
    uint64_t stop_ns = 0;
    uint64_t last_ns = 0;
    clockid_t clock_type;
    bool is_running = false;

    uint64_t nanoseconds() const { return clock_gettime_ns_adjusted(start_ns, clock_type); }

    inline uint64_t clock_gettime_ns_adjusted(uint64_t prev_time, clockid_t clock_type = CLOCK_MONOTONIC) const {
        uint64_t current_time = clock_gettime_ns(clock_type);
        if (prev_time <= current_time)
            return current_time;

        /// Something probably went completely wrong if time stepped back for more than 1 second.
        assert(prev_time - current_time <= 1000000000ULL);
        return prev_time;
    }

    inline uint64_t clock_gettime_ns(clockid_t clock_type = CLOCK_MONOTONIC) const {
        struct timespec ts {};
        clock_gettime(clock_type, &ts);
        return uint64_t(ts.tv_sec * 1000000000ULL + ts.tv_nsec);
    }
};