#pragma once

#include "util.h"
#include <condition_variable>
#include <exception>
#include <leveldb/status.h>
#include <memory>
#include <stdexcept>
#include <string>
#include <leveldb/db.h>
#include <leveldb/options.h>
#include <leveldb/slice.h>
#include <leveldb/write_batch.h>
#include <type_traits>

namespace Bench {

inline const size_t DEFAULT_VALUE_SIZE = 100;

// Comma-separated list of operations to run in the specified order
//   Actual benchmarks:
//      fillseq       -- write N values in sequential key order in async mode
//      fillrandom    -- write N values in random key order in async mode
//      overwrite     -- overwrite N values in random key order in async mode
//      fillsync      -- write N/100 values in random key order in sync mode
//      fill100K      -- write N/1000 100K values in random order in async mode
//      deleteseq     -- delete N keys in sequential order
//      deleterandom  -- delete N keys in random order
//      readseq       -- read N times sequentially
//      readreverse   -- read N times in reverse order
//      readrandom    -- read N times in random order
//      readmissing   -- read N missing keys in random order
//      readhot       -- read N times in random order from 1% section of DB
//      seekrandom    -- N random seeks
//      seekordered   -- N ordered seeks
//      open          -- cost of opening a DB
//      crc32c        -- repeated crc32c of 4K of data
//   Meta operations:
//      compact     -- Compact the entire DB
//      stats       -- Print DB stats
//      sstables    -- Print sstable info
//      heapprofile -- Dump a heap profile (if supported by this port)

enum BenchType {
    fillSeq,      // write N values in sequential key order in async mode
    fillrandom    // write N values in random key order in async mode
};

struct Statistics {
    // TODO
};

struct ThreadSharedArgs {
    std::mutex mu_;
    std::condition_variable cv_;
    size_t total_thread_num_;
    size_t running_thread_num_;

    leveldb::DB* db_;
    leveldb::WriteOptions write_option_;
    BenchType bench_type_;
    size_t entry_num_per_batch_;
    size_t entry_num;
    size_t value_size_;
};

struct ThreadArgs {
    Statistics stat_;
    size_t thread_id_;
    std::shared_ptr<ThreadSharedArgs> shared_args_;
};

struct BenchmarkParam {
    std::string db_data_dir_;
    size_t thread_num_;
    size_t entry_num_per_thread_;
    size_t value_size_;
    BenchType bench_type_;
};

class Benchmark {
public:
    Benchmark(const BenchmarkParam & param) : param_(param) {
        const std::string & db_data_dir = param.db_data_dir_;
        leveldb::Status status = leveldb::DestroyDB(db_data_dir, leveldb::Options());
        assert(status.ok());

        leveldb::Options options;
        options.create_if_missing = true;
        status = leveldb::DB::Open(options, db_data_dir, &db_);
        assert(status.ok());
    }

    void run();

private:
    leveldb::DB* db_;
    BenchmarkParam param_;
};

class KeyBuffer {
public:
    KeyBuffer() : append_pos_(0) {}

    void reset() { append_pos_ = 0; }

    template<typename T>
    void append(T item) {
        constexpr bool is_valid_type = std::is_integral_v<T> | std::is_same_v<char, T>;
        static_assert(is_valid_type);

        // Warning: appended position may exceed the array length
        // However, it's ok so far.
        T * dst = reinterpret_cast<T*>(&buffer_[append_pos_]);
        *dst = item;

        constexpr size_t type_len = sizeof(T);
        append_pos_ += type_len;
    }

    leveldb::Slice getSlice() { return leveldb::Slice(buffer_, append_pos_); }

private:
    char buffer_[1024];
    size_t append_pos_;
};

class ValueGenerator {
public:
    ValueGenerator() : pos_(0) {
        // We use a limited amount of data over and over again and ensure
        // that it is larger than the compression window (32KB), and also
        // large enough to serve all typical value sizes we want to write.
        RandString rander;
        data_ = rander.getRandString(1048576);
    }

    leveldb::Slice generate(size_t len) {
        if (pos_ + len > data_.size()) {
            pos_ = 0;
            assert(len < data_.size());
        }
        pos_ += len;
        return leveldb::Slice(data_.data() + pos_ - len, len);
    }

private:
    std::string data_;
    int pos_;
};

} // namespace
