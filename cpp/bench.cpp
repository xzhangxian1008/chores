#include "bench.h"
#include "Stopwatch.h"
#include "util.h"
#include <leveldb/options.h>
#include <leveldb/slice.h>
#include <memory>
#include <mutex>
#include <string>
#include <thread>

namespace Bench {

template<bool is_seq>
void writeBench(ThreadArgs * arg) {
    leveldb::DB * db = arg->shared_args_->db_;
    leveldb::WriteOptions write_option = arg->shared_args_->write_option_;
    leveldb::WriteBatch batch;
    KeyBuffer key_buf;
    ValueGenerator value_gen;
    size_t entry_num = arg->shared_args_->entry_num;
    size_t entry_num_per_batch = arg->shared_args_->entry_num_per_batch_;
    size_t value_size = arg->shared_args_->value_size_;
    RandNum num_rander(0, entry_num*10);
    int k;

    for (size_t i = 0; i < entry_num; i += entry_num_per_batch) {
        batch.Clear();
        for (size_t j =0; j < entry_num_per_batch; j++) {
            if constexpr (is_seq)
                k = i + j;
            else
                k = num_rander.getNum();

            key_buf.append(k);
            batch.Put(key_buf.getSlice(), value_gen.generate(value_size));
            key_buf.reset();
        }

        auto status = db->Write(write_option, &batch);
        if (!status.ok()) {
            std::fprintf(stderr, "put error: %s\n", status.ToString().c_str());
            std::exit(1);
        }
    }
}

void run_thread_impl(ThreadArgs * arg) {
    switch (arg->shared_args_->bench_type_) {
        case BenchType::fillSeq:
            writeBench<true>(arg);
            break;
        case BenchType::fillrandom:
            writeBench<false>(arg);
            break;
    }
}

void run_thread(void* arg) {
    ThreadArgs * thd_arg = reinterpret_cast<ThreadArgs*>(arg);
    std::shared_ptr<ThreadSharedArgs> shared_arg = thd_arg->shared_args_;

    {
        std::unique_lock<std::mutex> lock(shared_arg->mu_);
        shared_arg->running_thread_num_++;
        shared_arg->cv_.notify_all();
        shared_arg->cv_.wait(lock, [&]()->bool{
            return shared_arg->running_thread_num_ == shared_arg->total_thread_num_;
        });
    }

    run_thread_impl(thd_arg);

    {
        std::unique_lock<std::mutex> lock(shared_arg->mu_);
        shared_arg->running_thread_num_--;
        shared_arg->cv_.notify_all();
        shared_arg->cv_.wait(lock, [&]()->bool{
            return shared_arg->running_thread_num_ == 0;
        });
    }
}

void Benchmark::run() {
    size_t thread_num = param_.thread_num_;

    std::shared_ptr<ThreadSharedArgs> shared_args = std::make_shared<ThreadSharedArgs>();
    shared_args->total_thread_num_ = thread_num;
    shared_args->running_thread_num_ = 0;
    shared_args->bench_type_ = param_.bench_type_;
    shared_args->entry_num = param_.entry_num_per_thread_;

    if (param_.value_size_ == 0)
        shared_args->value_size_ = DEFAULT_VALUE_SIZE;
    else
        shared_args->value_size_ = param_.value_size_;

    switch (shared_args->bench_type_) {
        default:
            shared_args->entry_num_per_batch_ = 1;
    }

    std::vector<std::thread> threads;
    std::vector<ThreadArgs> thread_args;
    thread_args.resize(thread_num);
    for (size_t i = 0; i < thread_num; i++) {
        thread_args[i].shared_args_ = shared_args;
        thread_args[i].thread_id_ = i;
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
