import argparse
import pymysql
import threading
import time
import random
import datetime
import tpch
import config
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

def isTestFinished():
    lock.acquire()
    if isShutdown == True:
        lock.release()
        print("%s exit..." % threading.current_thread().name)
        return True
    lock.release()
    return False

# This function could handle errors raised by sqls and continue to run
def runErrorSqls(run_sqls):
    i = 0
    print("%s start..." % threading.current_thread().name)
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=target_database, cursorclass=pymysql.cursors.DictCursor)
    isFirst = False
    with connection:
        with connection.cursor() as cursor:
            while True:
                if isFirst == False:
                    cursor.execute("set tidb_enforce_mpp=1;")
                    cursor.execute("use %s;" % target_database)
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
    """explain analyze select l_orderkey from tpch10.lineitem group by l_orderkey having sum(l_quantity) > 300;"""
]

def getSql():
    idx = random.randint(1, len(tmp_sqls))
    return tmp_sqls[idx-1]

# Sqls run by this function should always success
def runNoErrorSqls():
    print("%s start..." % threading.current_thread().name)
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=target_database, cursorclass=pymysql.cursors.DictCursor)
    isFirst = False
    with connection:
        with connection.cursor() as cursor:
            while True:
                if isFirst == False:
                    # cursor.execute("set tidb_enforce_mpp=1;")
                    cursor.execute("use %s;" % target_database)
                    isFirst = True
                # idx = random.randint(1, len(tpch_sqls))
                # sql = tpch_sqls[idx].getSql()
                sql = getSql()
                start = time.time()
                cursor.execute(sql)
                end = time.time()
                # sqls[idx].addInfo(end - start)
                print("execution time: ", end - start)
                # result = cursor.fetchall()
                # print(result)

                if isTestFinished():
                    return

bookIds = [1, 30, 50]
taskId = 0
insertNum = 300000

project_name_or_id = [
    "project_name_or_id_10000000000000000000000000000000000000000000000000000000000",
    "project_name_or_id_11111111111111111111111111111111111111111111111111111111111",
    "project_name_or_id_22222222222222222222222222222222222222222222222222222222222",
    "project_name_or_id_33333333333333333333333333333333333333333333333333333333333",
    "project_name_or_id_44444444444444444444444444444444444444444444444444444444444",
    "project_name_or_id_55555555555555555555555555555555555555555555555555555555555",
    "project_name_or_id_66666666666666666666666666666666666666666666666666666666666",
    "project_name_or_id_77777777777777777777777777777777777777777777777777777777777",
    "project_name_or_id_88888888888888888888888888888888888888888888888888888888888",
    "project_name_or_id_99999999999999999999999999999999999999999999999999999999999"
]

token_address = [
    "token_address_10000000000000000000000000000000000000000000000000000000000",
    "token_address_11111111111111111111111111111111111111111111111111111111111",
    "token_address_22222222222222222222222222222222222222222222222222222222222",
    "token_address_33333333333333333333333333333333333333333333333333333333333",
    "token_address_44444444444444444444444444444444444444444444444444444444444",
    "token_address_55555555555555555555555555555555555555555555555555555555555",
    "token_address_66666666666666666666666666666666666666666666666666666666666",
    "token_address_77777777777777777777777777777777777777777777777777777777777",
    "token_address_88888888888888888888888888888888888888888888888888888888888",
    "token_address_99999999999999999999999999999999999999999999999999999999999"
]

address_or_general = "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"

def runInsertSqls():
    global lock
    global bookIds
    global taskId
    lock.acquire()
    # [startNum, endNum)
    startNum = taskId * insertNum
    endNum = (taskId + 1) * insertNum
    taskId += 1
    lock.release()
    connection = pymysql.connect(host=config.target_addr, port=config.target_port, user=config.target_user, database=target_database, cursorclass=pymysql.cursors.DictCursor)
    with connection:
        with connection.cursor() as cursor:
            i = startNum
            while i < endNum:
                idx = random.randint(0, 9)
                # insert_sql = "insert into matrix_address_labels values('%s', '%s')" % (address_or_general, address_or_general)
                insert_sql = "insert into all_projects values('%s', '%s', '%s')" % (project_name_or_id[idx], token_address[idx], project_name_or_id[idx])
                i += 1
                j = 1
                while j < 10000:
                    idx = random.randint(0, 9)
                    # insert_sql = "%s, ('%s', '%s')" % (insert_sql, address_or_general, address_or_general)
                    insert_sql = "%s, ('%s', '%s', '%s')" % (insert_sql, project_name_or_id[idx], token_address[idx], project_name_or_id[idx])
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
    args = parser.parse_args()
    return args

if __name__ == "__main__":
    print("Start Time: ", datetime.now())
    args = parseArgs()
    thread_num = args.thread_num
    test_time = args.test_time
    target_database = args.db

    threads = []
    while (thread_num > 0):
        thread = threading.Thread(target=runNoErrorSqls)
        thread.start()
        threads.append(thread)
        thread_num -= 1

    start = time.time()
    time.sleep(test_time)

    lock.acquire()
    isShutdown = True
    lock.release()

    for thread in threads:
        thread.join()

    # end = time.time()

    # count = 0
    # for sql in sqls.values():
    #     count += sql.getCount()
    #     print(sql.getInfo())

    # total_time = end - start

    # print("Total time: %f" % total_time)
    # print("Total queries: %d, QPS: %f" % (count, (count / (end - start))))
    print("End Time: ", datetime.now())
    print("exit...")
