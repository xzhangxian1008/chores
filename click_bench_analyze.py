import os

file = "bench_result"
query_num = 43
results = []

class QueryInfo:
    def __init__(self, id, times):
        self.id = id
        self.times = times

    def getAverageTime(self):
        time_sum = 0
        for time in self.times:
            time_sum += time
        return time_sum / len(self.times)

    def getMinTime(self):
        min_time = 9999999999
        for time in self.times:
            if time < min_time:
                min_time = time
        return min_time

    def getInfoAvg(self):
        return "Q%d: %f" % (self.id, self.getAverageTime())

    def getInfoMin(self):
        return "Q%d: %f" % (self.id, self.getMinTime())

def printQueryInfosAvg(infos):
    sum = 0
    for info in infos:
        print(info.getInfoAvg())
        sum += info.getAverageTime()
    print("Sum: %f" % sum)

def printQueryInfosMin(infos):
    sum = 0
    for info in infos:
        print(info.getInfoMin())
        sum += info.getMinTime()
    print("Sum: %f" % sum)

def analyze():
    with open(file, 'r') as f:
        data = f.readlines()
        if len(data) != query_num:
            exit(-1)
        analyzeImpl(data)

def analyzeImpl(data):
    query_id = 0
    for item in data:
        query_id += 1
        time = ""
        times = []
        for char in item:
            if char == '[':
                continue
            elif char == ',':
                times.append(float(time))
                time = ""
            elif char == ']':
                times.append(float(time))
                time = ""
                break
            else:
                time += char
        results.append(QueryInfo(query_id, times))

if __name__ == "__main__":
    analyze()
    printQueryInfosMin(results)
