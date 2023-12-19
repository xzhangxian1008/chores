import time
from time import gmtime, strftime
import subprocess

def getCurrentDateTimeStr():
    return strftime("%Y-%m-%d %H:%M:%S", gmtime())

class Info:
    def __init__(self, interval_time):
        self.reset()
        self.interval_time = interval_time
        self.log_file_name = "ping_file_%f" % interval_time
        with open(self.log_file_name, "w") as f:
            f.write("Start...\n")

    def reset(self):
        self.start_time = time.time()
        self.count = 0
        self.total_wait_time = 0
        self.max_time = 0
        self.min_time = 999999999
        self.fail_num = 0

    def updateMinAndMax(self, t):
        if t > self.max_time:
            self.max_time = t
        if t < self.min_time:
            self.min_time = t

    def getAvg(self):
        if self.count == 0:
            return 0
        return self.total_wait_time / self.count

    def recordFail(self):
        self.fail_num += 1

    def addTime(self, t):
        if t == -1:
            self.recordFail()
        else:
            self.count += 1
            self.total_wait_time += t
            self.updateMinAndMax(t)
        self.checkAndLog()

    def checkAndLog(self):
        now = time.time()
        elapsed_time = now - self.start_time
        if elapsed_time > self.interval_time:
            with open(self.log_file_name, "a") as f:
                currentTime = getCurrentDateTimeStr()
                content = "%s: count: %d, avg: %fs, max: %fs, min: %fs, fail: %d\n" % (currentTime, self.count, self.getAvg(), self.max_time, self.min_time, self.fail_num)
                f.write(content)
            self.reset()

class Detector:
    def __init__(self, ip):
        self.ip = ip
        self.cycle_30s_info = Info(30)
        self.cycle_10min_info = Info(60*10)
        self.cycle_30min_info = Info(60*30)

    def ping(self):
        start_time = time.time()
        ret = subprocess.run(["ping", "10.2.12.125", "-c 1"], stdout=subprocess.DEVNULL)
        if ret.returncode != 0:
            return -1
        end_time = time.time()
        return end_time - start_time

    # Infinity.
    def run(self):
        while True:
            elapsed_time = self.ping()
            self.cycle_30s_info.addTime(elapsed_time)
            self.cycle_10min_info.addTime(elapsed_time)
            self.cycle_30min_info.addTime(elapsed_time)
            time.sleep(0.3)

if __name__ == "__main__":
    detector = Detector("10.2.12.125")
    detector.run()
