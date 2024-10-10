#include "bench.h"
#include "Stopwatch.h"
#include <memory>
#include <thread>

namespace Bench {

void run_thread(void* arg) {

}

void Benchmark::run() {
    size_t thread_num = this->param_.thread_num_;
    
    std::shared_ptr<ThreadSharedArgs> shared_args = std::make_shared<ThreadSharedArgs>();
    shared_args->total_thread_num_ = thread_num;
    shared_args->initialized_thread_num_ = 0;

    std::vector<std::thread> threads;
    std::vector<ThreadArgs> thread_args;
    thread_args.resize(thread_num);
    for (size_t i = 0; i < thread_num; i++) {
        thread_args[i].shared_args_ = shared_args;
        threads.push_back(std::thread(run_thread, &thread_args[i]));
    }

    Stopwatch timer;
    timer.start();
    for (auto& thd : threads) {
        thd.join();
    }
    timer.stop();
    print("Total elapsed time: ", timer.elapsedSeconds(), "s");
}

} // namespace Bench
