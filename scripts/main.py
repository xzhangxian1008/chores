import argparse
import pymysql
import threading
import time
import random
import datetime
import tpch
import config
import util
from datetime import datetime
from sql import Sql
from sql import tpch_sqls
from sql import tpch1_spill_sqls
from sql import tpch10_spill_sqls

tmp_sql = """
explain analyze select l_orderkey from tpch10.lineitem group by l_orderkey having sum(l_quantity) > 300;
"""

lock = threading.Lock()
isShutdown = False
maximum_time = -1
minimum_time = 999999
time_collection = []

def isTestFinished():
    lock.acquire()
    if isShutdown == True:
        lock.release()
        print("%s exit..." % threading.current_thread().name)
        return True
    lock.release()
    return False

def updateExecutionTime(exec_time):
    global lock
    global maximum_time
    global minimum_time
    global time_collection
    lock.acquire()
    if exec_time > maximum_time:
        maximum_time = exec_time
    if exec_time < minimum_time:
        minimum_time = exec_time
    time_collection.append(exec_time)
    lock.release()

def printExecutionTime():
    global maximum_time
    global minimum_time
    global time_collection
    if len(time_collection) == 0:
        return

    avg_time = 0
    for item in time_collection:
        avg_time += item
    avg_time /= len(time_collection)
    print("avg: %.2f, maximum: %.2f, minimum: %.2f." % (avg_time, maximum_time, minimum_time))

# This function could handle errors raised by sqls and continue to run
def runErrorSqls(run_sqls):
    i = 0
    print("%s start..." % threading.current_thread().name)
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=config.target_database, cursorclass=pymysql.cursors.DictCursor)
    isFirst = False
    with connection:
        with connection.cursor() as cursor:
            while True:
                if isFirst == False:
                    cursor.execute("set tidb_enforce_mpp=1;")
                    cursor.execute("use %s;" % config.target_database)
                    isFirst = True
                idx = random.randint(1, len(run_sqls))
                sql = run_sqls[idx].getSql()
                start = time.time()
                try:
                    cursor.execute(sql)
                except Exception as e:
                    # print("Exception happens when executing query.", e)
                    pass
                end = time.time()
                print("%s: %d, execution's finish time: %s, elapsed time: %f" % (threading.current_thread().name, i, datetime.now(), end - start))
                i += 1

                if isTestFinished():
                    return

tmp_sqls = [
    # """explain analyze select l_orderkey from tpch10.lineitem group by l_orderkey having sum(l_quantity) > 300;"""
    # """explain analyze select l_discount from tpch10.lineitem group by l_discount;"""
    """explain analyze select count(distinct l_partkey) from tpch10.lineitem group by l_orderkey having sum(l_quantity) > 300;"""
    # """explain analyze select count(distinct l_partkey) from tpch10.lineitem group by l_discount;"""
]

def getSql():
    idx = random.randint(1, len(tmp_sqls))
    return tmp_sqls[idx-1]

def executeSQL(sql):
    results = []
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=config.target_database, cursorclass=pymysql.cursors.DictCursor)
    with connection:
        with connection.cursor() as cursor:
            cursor.execute(sql)
            all_data = cursor.fetchall()
            result_num = len(all_data)
            i = 0
            while i < result_num:
                result = ""
                for _, value in all_data[i].items():
                    if len(result) == 0:
                        result = str(value)
                    else:
                        result = "%s, %s" % (result, str(value))
                results.append(result)
                i += 1

    results.sort()
    with open("sql_result.txt", "w") as f:
        for result in results:
            f.write("%s\n" % result)

# Sqls run by this function should always success
def runNoErrorSqls():
    print("%s start..." % threading.current_thread().name)
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=config.target_database, cursorclass=pymysql.cursors.DictCursor)
    isFirst = False
    with connection:
        with connection.cursor() as cursor:
            while True:
                if isFirst == False:
                    # cursor.execute("set tidb_enforce_mpp=1;")
                    cursor.execute("use %s;" % config.target_database)
                    isFirst = True
                # idx = random.randint(1, len(tpch_sqls))
                # sql = tpch_sqls[idx].getSql()
                sql = getSql()
                start = time.time()
                cursor.execute(sql)
                end = time.time()
                # sqls[idx].addInfo(end - start)
                execution_time = end - start
                print("execution time: ", execution_time)
                # result = cursor.fetchall()
                # print(result)

                updateExecutionTime(execution_time)

                if isTestFinished():
                    return


taskId = 0
insertNum = 100000

def runInsertSqls():
    global lock
    global taskId
    lock.acquire()
    # Range: [startNum, endNum)
    startNum = taskId * insertNum
    endNum = (taskId + 1) * insertNum
    taskId += 1
    lock.release()
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=config.target_database, cursorclass=pymysql.cursors.DictCursor)
    with connection:
        with connection.cursor() as cursor:
            i = startNum
            while i < endNum:
                insert_sql = "insert into t1 values %s" % util.generateValue()
                i += 1
                j = 1
                while j < 10000:
                    insert_sql = "%s, %s" % (insert_sql, util.generateValue())
                    i += 1
                    j += 1
                insert_sql += ';'
                cursor.execute(insert_sql)
                print("Ready to commit")
                connection.commit()
                print("Commit done")


def runErrorSqlsTmp():
    tmps = {
        1: Sql(tmp_sql, "tmp sql")
    }
    runErrorSqls(tmps)

def runRandomFailPointTest():
    print("Run random fail point test...")
    runErrorSqls(tpch_sqls)
    # runErrorSqls(tpch10_spill_sqls)
    # runErrorSqlsTmp()

def runTPCH():
    print("Run tpch...")
    runNoErrorSqls()

def parseArgs():
    description = "You should add those parameter"
    parser = argparse.ArgumentParser(description=description)
    parser.add_argument('--thread_num', type=int, default=1, help='thread number, default 1')
    parser.add_argument('--test_time', type=int, default=0, help="test time(seconds), default 30s")
    parser.add_argument('--db', type=str, default="test", help="database, default 'test'")
    parser.add_argument('--port', type=int, default=7001, help="port, default=7001")
    parser.add_argument('--address', type=str, default="127.0.0.1", help="ipv4 address, default=127.0.0.1")
    args = parser.parse_args()
    return args

def runInMode1():
    global isShutdown
    global thread_num
    threads = []
    while (thread_num > 0):
        thread = threading.Thread(target=runInsertSqls)
        thread.start()
        threads.append(thread)
        thread_num -= 1

    time.sleep(test_time)

    lock.acquire()
    isShutdown = True
    lock.release()

    for thread in threads:
        thread.join()

# python main.py --thread_num=1 --port=7001
if __name__ == "__main__":
    print("Start Time: ", datetime.now())
    args = parseArgs()
    thread_num = args.thread_num
    test_time = args.test_time
    config.target_addr = args.address
    config.target_port = args.port
    config.target_database = args.db

    print("host: %s, port: %d, database: %s, test_time: %ds, thread_num: %d" % (config.target_addr, config.target_port, config.target_database, test_time, thread_num))

    runInMode1()
    # runInMode2()

    printExecutionTime()
    print("End Time: ", datetime.now())
    print("exit...")
