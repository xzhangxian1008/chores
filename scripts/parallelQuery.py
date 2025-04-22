import pymysql
import threading
import time

concurrency = 1 # 并发数
test_time = 2 # 测试持续时间(单位: 秒)

host_addr = "" # 地址
host_port = 7001  # 端口
user = ""  # 用户
password = ""  # 密码
database = "test" # 数据库
sql = "explain analyze select sum(v) from agg group by p" # sql

lock = threading.Lock()
global_thread_id = 0
isShutdown = False

def isTestFinished():
    global lock
    lock.acquire()
    if isShutdown == True:
        lock.release()
        print("%s exit..." % threading.current_thread().name)
        return True
    lock.release()
    return False

def run():
    global host_addr
    global host_port
    global user
    global password
    global database
    global sql
    global lock
    global global_thread_id
    lock.acquire()
    thread_id = global_thread_id
    global_thread_id += 1
    lock.release()
    print("Thread %d is running..." % thread_id)

    try:
        connection = pymysql.connect(host=host_addr, port=host_port, user=user, password=password, database=database, cursorclass=pymysql.cursors.DictCursor)
        with connection:
            with connection.cursor() as cursor:
                while True:
                    start = time.time()
                    cursor.execute(sql)
                    end = time.time()
                    execution_time = end - start
                    print("Execution time: %fs" % execution_time)

                    if isTestFinished():
                        print("Thread %d exists" % thread_id)
                        return
    except Exception as e:
        print(e)

if __name__ == "__main__":
    print("Start to run sqls")
    
    i = 0
    threads = []
    while i < concurrency:
        thread = threading.Thread(target=run)
        thread.start()
        threads.append(thread)
        i += 1

    start = time.time()
    time.sleep(test_time)

    lock.acquire()
    isShutdown = True
    lock.release()

    for thread in threads:
        thread.join()

    print("Test done")
