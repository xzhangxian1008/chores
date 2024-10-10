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

namespace Bench {

struct Statistics {
    // TODO
};

struct ThreadSharedArgs {
    std::mutex mu_;
    std::condition_variable cv_;
    size_t total_thread_num_;
    size_t initialized_thread_num_;
};

struct ThreadArgs {
    Statistics stat_;
    std::shared_ptr<ThreadSharedArgs> shared_args_;
};

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
    FillSeq
};

struct BenchmarkParam {
    std::string db_data_dir_;
    size_t thread_num_;
    size_t op_num_;
    BenchType bench_type_;
};

class Benchmark {
public:
    Benchmark(const BenchmarkParam & param) : param_(param) {
        const std::string & db_data_dir = param.db_data_dir_;
        leveldb::Status status = leveldb::DestroyDB(db_data_dir, leveldb::Options());
        if (!status.ok()) {
            throw std::runtime_error("Fail to destroy database");
        }

        leveldb::Options options;
        options.create_if_missing = true;
        status = leveldb::DB::Open(options, db_data_dir, &db_);
        assert(status.ok());
        if (!status.ok()) {
            throw std::runtime_error("Fail to create database");
        }
    }

    void run();

private:
    leveldb::DB* db_;
    BenchmarkParam param_;
};

} // namespace
