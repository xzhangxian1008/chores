#include "bench.h"
#include <leveldb/db.h>
#include <leveldb/options.h>

const std::string leveldb_data_file = "/DATA/disk3/xzx/tmp/leveldb_data/testdb";

int main() {
    Bench::KeyBuffer buf;
    buf.append(95);
    auto slice = buf.getSlice();
    print(slice.ToString());
    
    // Bench::BenchmarkParam param;
    // param.thread_num_ = 5;

    // Bench::Benchmark bench(param);
    // bench.run();
}
