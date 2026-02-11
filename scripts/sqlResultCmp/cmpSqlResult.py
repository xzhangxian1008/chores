# test sqls:
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 0;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 0 offset 100;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 10;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 500;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 1000;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 5000;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 10 offset 100;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 1000 offset 10;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 500 offset 100;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 5000 offset 1000;
# select /*+ use_index(t1, idx1) */ c0 from t1 order by c0 limit 3000 offset 4000;

def cmp(expectFileName, actualFileName):
    print("Start to compare %s and %s", expectFileName, actualFileName)
    
    with open(expectFileName, "r") as f:
        file1ResultRows = f.readlines()

    with open(actualFileName, "r") as f:
        file2ResultRows = f.readlines()

    if len(file1ResultRows) != len(file2ResultRows):
        raise Exception("length not equal, %d vs %d" % (len(file1ResultRows), len(file2ResultRows)))

    # file1.sort()
    # file2.sort()
    length = len(file1ResultRows)
    i = 0
    while i < length:
        if file1ResultRows[i] != file2ResultRows[i]:
            print("error: i %d, <%s> vs <%s>" % (i, file1ResultRows[i], file2ResultRows[i]))
            raise Exception("Incorrect answer")
        i += 1

    print("Success!")

if __name__ == "__main__":
    expectResultFiles = ["expect1.txt", "expect2.txt", "expect3.txt", "expect4.txt", "expect5.txt", "expect6.txt", "expect7.txt", "expect8.txt", "expect9.txt", "expect10.txt", "expect11.txt"]
    actualResultFiles = ["actual1.txt", "actual2.txt", "actual3.txt", "actual4.txt", "actual5.txt", "actual6.txt", "actual7.txt", "actual8.txt", "actual9.txt", "actual10.txt", "actual11.txt"]

    caseCount = len(expectResultFiles)
    i = 0
    while i < caseCount:
        cmp(expectResultFiles[i], actualResultFiles[i])
        i += 1
